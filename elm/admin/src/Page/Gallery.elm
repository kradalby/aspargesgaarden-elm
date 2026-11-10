module Page.Gallery exposing (Model, Msg, init, isDirty, subscriptions, update, view)

{-| "Galleri": the photos on /galleri, in order. Every change is saved at
once (there is no save button to forget); what could not be saved says so
next to the photo, with a way to try again.
-}

import Api exposing (Photo, UsedBy)
import Dict exposing (Dict)
import Form
import Html exposing (Html, div, li, p, span, text, ul)
import Html.Attributes as A exposing (attribute, class)
import Html.Events exposing (on, onInput, targetValue)
import Json.Decode as D
import Photos
import Text
import Ui


type alias Model =
    { loading : Loading
    , order : List Int
    , savedOrder : List Int
    , orderSaving : Bool
    , orderFailed : Maybe Api.Error
    , alts : Dict Int String
    , saves : Dict Int Save
    , photos : Photos.Model
    , delete : Maybe Delete
    , announce : String
    }


type Loading
    = Loading
    | LoadFailed Api.Error
    | Ready


type Save
    = Saving
    | Saved
    | SaveFailed Api.Error


type alias Delete =
    { id : Int, state : DeleteState }


type DeleteState
    = Asking
    | Deleting
    | InUse (List UsedBy)
    | DeleteFailed Api.Error


init : ( Model, Cmd Msg )
init =
    let
        ( photos, cmd ) =
            Photos.init
    in
    ( { loading = Loading
      , order = []
      , savedOrder = []
      , orderSaving = False
      , orderFailed = Nothing
      , alts = Dict.empty
      , saves = Dict.empty
      , photos = photos
      , delete = Nothing
      , announce = ""
      }
    , Cmd.batch [ Api.getGallery Loaded, Cmd.map PhotosMsg cmd ]
    )


isDirty : Model -> Bool
isDirty model =
    model.order
        /= model.savedOrder
        || not (List.isEmpty model.photos.uploads)
        || List.any (\( id, alt ) -> Just alt /= Maybe.map .alt (Photos.get id model.photos)) (Dict.toList model.alts)


type Msg
    = Loaded (Result Api.Error (List Photo))
    | RetryLoad
    | PhotosMsg Photos.Msg
    | Move Int Int
    | OrderSaved (Result Api.Error (List Photo))
    | RetryOrder
    | SetAlt Int String
    | SaveAlt Int
    | SetPhotographer Int (Maybe Int)
    | PhotoSaved Int (Result Api.Error Photo)
    | AskDelete Int
    | ConfirmDelete
    | Deleted Int (Result Api.Error ())
    | RemoveFromGallery Int
    | CloseDelete
    | Ignored


update : Msg -> Model -> ( Model, Cmd Msg )
update msg model =
    case msg of
        Loaded (Ok photos) ->
            ( { model | loading = Ready, order = List.map .id photos, savedOrder = List.map .id photos, photos = Photos.add photos model.photos }, Cmd.none )

        Loaded (Err e) ->
            ( { model | loading = LoadFailed e }, Cmd.none )

        RetryLoad ->
            ( { model | loading = Loading }, Api.getGallery Loaded )

        PhotosMsg pm ->
            let
                ( photos, cmd, outs ) =
                    Photos.update pm model.photos
            in
            List.foldl applyOut ( { model | photos = photos }, Cmd.map PhotosMsg cmd ) outs

        Move i d ->
            let
                order =
                    Form.move i d model.order
            in
            saveOrder order
                ( { model | announce = Text.movedTo (clamp 1 (List.length order) (i + d + 1)) }
                , focus (Ui.moveId (i + d) d)
                )

        OrderSaved (Ok photos) ->
            ( { model | orderSaving = False, orderFailed = Nothing, savedOrder = List.map .id photos, photos = Photos.add photos model.photos }, Cmd.none )

        OrderSaved (Err e) ->
            ( { model | orderSaving = False, orderFailed = Just e }, Cmd.none )

        RetryOrder ->
            saveOrder model.order ( model, Cmd.none )

        SetAlt id s ->
            ( { model | alts = Dict.insert id s model.alts, saves = Dict.remove id model.saves }, Cmd.none )

        SaveAlt id ->
            savePhoto id (photographerOf id model) model

        SetPhotographer id ph ->
            savePhoto id ph model

        PhotoSaved id (Ok photo) ->
            ( { model
                | photos = Photos.add [ photo ] model.photos
                , saves = Dict.insert id Saved model.saves
                , alts =
                    -- Keep typing that happened while the save was out.
                    if Dict.get id model.alts == Just photo.alt then
                        Dict.remove id model.alts

                    else
                        model.alts
                , announce = "Lagret."
              }
            , Cmd.none
            )

        PhotoSaved id (Err e) ->
            ( { model | saves = Dict.insert id (SaveFailed e) model.saves, announce = "Ikke lagret. " ++ Text.error e }, Cmd.none )

        AskDelete id ->
            ( { model | delete = Just { id = id, state = Asking } }, Cmd.none )

        ConfirmDelete ->
            case model.delete of
                Just d ->
                    ( { model | delete = Just { d | state = Deleting } }, Api.deletePhoto d.id (Deleted d.id) )

                Nothing ->
                    ( model, Cmd.none )

        Deleted id (Ok ()) ->
            ( { model
                | delete = Nothing
                , order = List.filter ((/=) id) model.order
                , savedOrder = List.filter ((/=) id) model.savedOrder
                , announce = "Bildet er slettet."
              }
            , focus "legg-til-bilder"
            )

        Deleted id (Err (Api.PhotoInUse used)) ->
            ( { model | delete = Just { id = id, state = InUse used } }, Cmd.none )

        Deleted id (Err e) ->
            ( { model | delete = Just { id = id, state = DeleteFailed e } }, Cmd.none )

        RemoveFromGallery id ->
            saveOrder (List.filter ((/=) id) model.order)
                ( { model | delete = Nothing, announce = "Bildet er tatt ut av galleriet." }, focus "legg-til-bilder" )

        CloseDelete ->
            ( { model | delete = Nothing }
            , model.delete |> Maybe.map (\d -> focus ("slett-" ++ String.fromInt d.id)) |> Maybe.withDefault Cmd.none
            )

        Ignored ->
            ( model, Cmd.none )


applyOut : Photos.Out -> ( Model, Cmd Msg ) -> ( Model, Cmd Msg )
applyOut out ( model, cmd ) =
    case out of
        Photos.Added _ p ->
            if List.member p.id model.order then
                ( model, cmd )

            else
                saveOrder (model.order ++ [ p.id ]) ( model, cmd )

        Photos.Credit id ph ->
            let
                ( m, c ) =
                    savePhoto id ph model
            in
            ( m, Cmd.batch [ cmd, c ] )


saveOrder : List Int -> ( Model, Cmd Msg ) -> ( Model, Cmd Msg )
saveOrder order ( model, cmd ) =
    ( { model | order = order, orderSaving = True, orderFailed = Nothing }
    , Cmd.batch [ cmd, Api.setGallery order OrderSaved ]
    )


altOf : Int -> Model -> String
altOf id model =
    case Dict.get id model.alts of
        Just s ->
            s

        Nothing ->
            Photos.get id model.photos |> Maybe.map .alt |> Maybe.withDefault ""


photographerOf : Int -> Model -> Maybe Int
photographerOf id model =
    Photos.get id model.photos |> Maybe.andThen .photographer |> Maybe.map .id


savePhoto : Int -> Maybe Int -> Model -> ( Model, Cmd Msg )
savePhoto id ph model =
    ( { model | saves = Dict.insert id Saving model.saves }
    , Api.updatePhoto id { alt = String.trim (altOf id model), photographerId = ph } (PhotoSaved id)
    )


focus : String -> Cmd Msg
focus =
    Ui.focus Ignored


subscriptions : Model -> Sub Msg
subscriptions model =
    Sub.map PhotosMsg (Photos.subscriptions model.photos)



-- VIEW


view : Model -> Html Msg
view model =
    let
        photos =
            List.filterMap (\id -> Photos.get id model.photos) model.order

        summary =
            Photos.busySummary photos model.photos
    in
    div [ class "flex flex-col gap-8" ]
        [ div [ class "flex flex-wrap gap-4 items-center justify-between" ]
            [ Ui.h1 "Galleri"
            , Ui.primaryButton [ A.id "legg-til-bilder" ] (PhotosMsg (Photos.pick Photos.Gallery True)) "+ Legg til bilder"
            ]
        , Ui.help [] "Bildene vises på galleri-siden i denne rekkefølgen. Alt du endrer her, lagres med en gang."
        , Ui.liveRegion model.announce
        , Photos.liveRegion model.photos
        , if summary == "" then
            text ""

          else
            div [ class "rounded-lg border-2 border-mose bg-lys-eukalyptus/20 p-4 font-bold", A.id "galleri-status" ] [ text summary ]
        , case model.orderFailed of
            Just e ->
                Ui.notice "error"
                    [ p [ attribute "role" "alert" ] [ text ("Rekkefølgen ble ikke lagret. " ++ Text.error e) ]
                    , div [] [ Ui.primaryButton [] RetryOrder "Prøv igjen" ]
                    ]

            Nothing ->
                text ""
        , div [ class "flex flex-col gap-2" ] (List.map (Photos.uploadRow PhotosMsg) (Photos.uploadsFor Photos.Gallery model.photos))
        , case model.loading of
            Loading ->
                Ui.spinnerText "Henter bildene …"

            LoadFailed e ->
                Ui.notice "error"
                    [ p [ attribute "role" "alert" ] [ text ("Klarte ikke å hente galleriet. " ++ Text.error e) ]
                    , div [] [ Ui.primaryButton [] RetryLoad "Prøv igjen" ]
                    ]

            Ready ->
                if List.isEmpty photos then
                    p [ class "text-brown-text italic" ] [ text "Galleriet er tomt. Trykk «Legg til bilder» for å legge inn bilder." ]

                else
                    ul [ class "grid gap-5 sm:grid-cols-2 xl:grid-cols-3" ] (List.indexedMap (photoCard model (List.length photos)) photos)
        , deleteDialog model
        , Photos.dialog PhotosMsg model.photos
        ]


photoCard : Model -> Int -> Int -> Photo -> Html Msg
photoCard model n i photo =
    let
        altId =
            "alt-" ++ String.fromInt photo.id

        alt =
            altOf photo.id model
    in
    li [ class "bg-white border border-brown rounded-xl p-4 flex flex-col gap-3", attribute "data-photo" (String.fromInt photo.id) ]
        [ Photos.image [ class "w-full aspect-[4/3] rounded-lg" ] photo
        , p [ class "font-bold" ] [ text ("Bilde " ++ String.fromInt (i + 1) ++ " av " ++ String.fromInt n) ]
        , Photos.statusView PhotosMsg photo
        , Photos.sizeNote photo
        , Ui.field
            { id = altId
            , label = "Beskriv bildet"
            , help = "Lagres når du går videre fra feltet."
            , errors = []
            , control = Ui.input [ A.value alt, onInput (SetAlt photo.id), on "change" (D.map (\_ -> SaveAlt photo.id) targetValue) ]
            }
        , if String.trim alt == "" then
            p [ class "flex gap-2 items-center" ]
                [ span [ attribute "aria-hidden" "true", class "font-bold" ] [ text "!" ], text "Mangler beskrivelse. Den leses opp for blinde og hjelper Google." ]

          else
            text ""
        , case Dict.get photo.id model.saves of
            Just Saving ->
                Ui.spinnerText "Lagrer …"

            Just Saved ->
                p [ class "text-mork-eukalyptus font-bold" ] [ span [ attribute "aria-hidden" "true" ] [ text "✓ " ], text "Lagret" ]

            Just (SaveFailed e) ->
                div [ class "flex flex-col gap-2" ]
                    [ Ui.errorText ("lagre-feil-" ++ String.fromInt photo.id) [ "Ikke lagret. " ++ Text.error e ]
                    , div [] [ Ui.secondaryButton [] (SaveAlt photo.id) "Prøv igjen" ]
                    ]

            Nothing ->
                text ""
        , Photos.photographerField
            { photoId = photo.id
            , selected = photographerOf photo.id model
            , onSelect = SetPhotographer photo.id
            , toMsg = PhotosMsg
            }
            model.photos
        , div [ class "flex flex-wrap gap-2 mt-auto pt-2" ]
            [ Ui.secondaryButton [ A.id (Ui.moveId i -1), A.disabled (i == 0), attribute "aria-label" ("Flytt bilde " ++ String.fromInt (i + 1) ++ " tidligere") ] (Move i -1) "← Tidligere"
            , Ui.secondaryButton [ A.id (Ui.moveId i 1), A.disabled (i == n - 1), attribute "aria-label" ("Flytt bilde " ++ String.fromInt (i + 1) ++ " senere") ] (Move i 1) "Senere →"
            , Ui.dangerButton [ A.id ("slett-" ++ String.fromInt photo.id), attribute "aria-label" ("Slett bilde " ++ String.fromInt (i + 1)) ] (AskDelete photo.id) "Slett"
            ]
        ]


deleteDialog : Model -> Html Msg
deleteDialog model =
    case model.delete of
        Nothing ->
            text ""

        Just d ->
            Ui.modal { id = "slett-bilde", title = "Slette bildet?", onClose = CloseDelete, wide = False } <|
                case d.state of
                    InUse used ->
                        [ p [ class "text-lg" ]
                            [ text
                                ("Dette bildet brukes også i "
                                    ++ String.join " og " (List.map (\u -> "«" ++ u.title ++ "»") used)
                                    ++ ", så det kan ikke slettes helt."
                                )
                            ]
                        , p [ class "text-lg" ] [ text "Du kan ta det ut av galleriet i stedet. Da blir det liggende i arrangementet." ]
                        , div [ class "flex flex-col sm:flex-row gap-3 justify-end" ]
                            [ Ui.secondaryButton [] CloseDelete "Avbryt"
                            , Ui.primaryButton [ A.autofocus True ] (RemoveFromGallery d.id) "Ta ut av galleriet"
                            ]
                        ]

                    _ ->
                        [ p [ class "text-lg" ] [ text "Bildet blir borte fra galleriet for godt. Det kan ikke angres." ]
                        , case d.state of
                            DeleteFailed e ->
                                Ui.notice "error" [ p [ attribute "role" "alert" ] [ text (Text.error e) ] ]

                            _ ->
                                text ""
                        , div [ class "flex flex-col sm:flex-row gap-3 justify-end" ]
                            [ Ui.secondaryButton [ A.autofocus True ] CloseDelete "Nei, behold det"
                            , Ui.dangerButton [ A.disabled (d.state == Deleting) ]
                                ConfirmDelete
                                (if d.state == Deleting then
                                    "Sletter …"

                                 else
                                    "Ja, slett"
                                )
                            ]
                        ]
