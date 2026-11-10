module Photos exposing
    ( Model
    , Msg
    , Out(..)
    , Upload
    , add
    , busySummary
    , creditText
    , dialog
    , get
    , image
    , init
    , isPending
    , liveRegion
    , photographerField
    , pick
    , statusView
    , subscriptions
    , update
    , uploadRow
    , uploadsFor
    )

{-| Everything about photos that the event editor and the gallery share:
uploading with progress, following the server while it prepares the sizes,
and the photographer credit with its small add-or-correct dialog.

A photo goes through: uploading (bytes) → checking (the server reads it) →
preparing (done of total sizes) → ready, or failed with a reason. Each state
is shown as words, so nothing depends on colour, and changes are announced
in a polite live region.

-}

import Api exposing (Photo, PhotoStatus, Photographer)
import Browser.Dom as Dom
import Dict exposing (Dict)
import File exposing (File)
import File.Select
import Html exposing (Html, div, img, label, option, p, span, text)
import Html.Attributes as A exposing (class)
import Html.Events exposing (onInput)
import Html.Keyed as Keyed
import Http
import Task
import Text
import Time
import Ui


type alias Model =
    { photos : Dict Int Photo
    , uploads : List Upload
    , next : Int
    , photographers : List Photographer
    , photographersFailed : Bool
    , dialog : Maybe Dialog
    , announce : String
    , polling : Bool

    -- Bumped to rebuild the photographer selects, so a "new photographer"
    -- choice that was cancelled does not stay selected in the DOM.
    , selectKey : Int
    }


type alias Upload =
    { key : Int
    , slot : String
    , file : File
    , sent : Int
    , size : Int
    , state : UploadState
    }


type UploadState
    = Sending
    | Checking
    | Failed Api.Error


type alias Dialog =
    { forPhoto : Int
    , editing : Maybe Int
    , name : String
    , url : String
    , saving : Bool
    , error : Maybe Api.Error
    }


{-| What the page has to act on.
-}
type Out
    = Added String Photo
    | Credit Int (Maybe Int)


init : ( Model, Cmd Msg )
init =
    ( { photos = Dict.empty
      , uploads = []
      , next = 0
      , photographers = []
      , photographersFailed = False
      , dialog = Nothing
      , announce = ""
      , polling = False
      , selectKey = 0
      }
    , Api.getPhotographers GotPhotographers
    )


{-| Remember photos the page shows, so their state is followed.
-}
add : List Photo -> Model -> Model
add photos model =
    { model
        | photos = List.foldl (\p -> Dict.insert p.id p) model.photos photos
        , photographers =
            List.foldl
                (\p acc ->
                    case p.photographer of
                        Just ph ->
                            if List.any (\x -> x.id == ph.id) acc then
                                acc

                            else
                                acc ++ [ ph ]

                        Nothing ->
                            acc
                )
                model.photographers
                photos
    }


get : Int -> Model -> Maybe Photo
get id model =
    Dict.get id model.photos


isPending : Photo -> Bool
isPending p =
    p.status.state == "preparing" || (not p.status.previewReady && p.status.state /= "failed")


pendingIds : Model -> List Int
pendingIds model =
    Dict.values model.photos |> List.filter isPending |> List.map .id


uploadsFor : String -> Model -> List Upload
uploadsFor slot model =
    List.filter (\u -> u.slot == slot) model.uploads



-- UPDATE


type Msg
    = Pick String Bool
    | Picked String File (List File)
    | Progress Int Http.Progress
    | Uploaded Int (Result Api.Error Photo)
    | Cancel Int
    | Retry Int
    | Dismiss Int
    | Poll
    | GotStatuses (Result Api.Error (Dict Int PhotoStatus))
    | Prepare Int
    | Prepared Int (Result Api.Error PhotoStatus)
    | GotPhotographers (Result Api.Error (List Photographer))
    | OpenPhotographer Int (Maybe Photographer)
    | DialogName String
    | DialogUrl String
    | SavePhotographer
    | SavedPhotographer (Result Api.Error Photographer)
    | UseExisting Photographer
    | CloseDialog
    | Focused


{-| Opens the file chooser for slot ("main", "extra", "gallery").
-}
pick : String -> Bool -> Msg
pick =
    Pick


tracker : Int -> String
tracker key =
    "upload-" ++ String.fromInt key


send : Upload -> Cmd Msg
send u =
    Api.upload (tracker u.key) u.file (Uploaded u.key)


update : Msg -> Model -> ( Model, Cmd Msg, List Out )
update msg model =
    case msg of
        Pick slot many ->
            ( model
            , if many then
                File.Select.files [ "image/*" ] (Picked slot)

              else
                File.Select.file [ "image/*" ] (\f -> Picked slot f [])
            , []
            )

        Picked slot f fs ->
            let
                files =
                    f :: fs

                new =
                    List.indexedMap
                        (\i file ->
                            { key = model.next + i, slot = slot, file = file, sent = 0, size = File.size file, state = Sending }
                        )
                        files
            in
            ( { model
                | uploads = model.uploads ++ new
                , next = model.next + List.length new
                , announce =
                    if List.length new == 1 then
                        "Laster opp " ++ File.name f ++ " …"

                    else
                        "Laster opp " ++ String.fromInt (List.length new) ++ " bilder …"
              }
            , Cmd.batch (List.map send new)
            , []
            )

        Progress key p ->
            ( mapUpload key
                (\u ->
                    case p of
                        Http.Sending s ->
                            { u
                                | sent = s.sent
                                , size = s.size
                                , state =
                                    if s.sent >= s.size then
                                        Checking

                                    else
                                        Sending
                            }

                        Http.Receiving _ ->
                            { u | state = Checking }
                )
                model
            , Cmd.none
            , []
            )

        Uploaded key result ->
            case ( List.filter (\u -> u.key == key) model.uploads, result ) of
                ( [ u ], Ok photo ) ->
                    ( add [ photo ] { model | uploads = List.filter (\x -> x.key /= key) model.uploads, announce = File.name u.file ++ " er lastet opp. Bildet klargjøres nå." }
                    , Cmd.none
                    , [ Added u.slot photo ]
                    )

                ( [ u ], Err e ) ->
                    ( mapUpload key (\x -> { x | state = Failed e }) { model | announce = File.name u.file ++ ": " ++ uploadError e }
                    , Cmd.none
                    , []
                    )

                _ ->
                    -- Cancelled or dismissed meanwhile.
                    ( model, Cmd.none, [] )

        Cancel key ->
            ( { model | uploads = List.filter (\u -> u.key /= key) model.uploads, announce = "Opplastingen er avbrutt." }
            , Http.cancel (tracker key)
            , []
            )

        Retry key ->
            case List.filter (\u -> u.key == key) model.uploads of
                [ u ] ->
                    ( mapUpload key (\x -> { x | state = Sending, sent = 0 }) { model | announce = "Prøver igjen med " ++ File.name u.file ++ " …" }
                    , send u
                    , []
                    )

                _ ->
                    ( model, Cmd.none, [] )

        Dismiss key ->
            ( { model | uploads = List.filter (\u -> u.key /= key) model.uploads }, Cmd.none, [] )

        Poll ->
            case pendingIds model of
                [] ->
                    ( model, Cmd.none, [] )

                ids ->
                    ( { model | polling = True }, Api.getStatuses (List.take 100 ids) GotStatuses, [] )

        GotStatuses (Ok statuses) ->
            let
                changed =
                    Dict.toList statuses
                        |> List.filterMap
                            (\( id, s ) ->
                                Dict.get id model.photos
                                    |> Maybe.andThen
                                        (\p ->
                                            if p.status.state /= s.state then
                                                Just ( p, s )

                                            else
                                                Nothing
                                        )
                            )

                announce =
                    changed
                        |> List.map
                            (\( p, s ) ->
                                case s.state of
                                    "ready" ->
                                        name p ++ " er klart."

                                    "failed" ->
                                        name p ++ ": " ++ Text.photoFailure s.reason

                                    _ ->
                                        ""
                            )
                        |> String.join " "
            in
            ( { model
                | polling = False
                , photos = Dict.foldl (\id s -> Dict.update id (Maybe.map (\p -> { p | status = s }))) model.photos statuses
                , announce =
                    if announce == "" then
                        model.announce

                    else
                        announce
              }
            , Cmd.none
            , []
            )

        GotStatuses (Err _) ->
            -- The next tick asks again; a phone on a weak network catches up.
            ( { model | polling = False }, Cmd.none, [] )

        Prepare id ->
            ( setStatus id (\s -> { s | state = "preparing", reason = "" }) model
            , Api.prepare id (Prepared id)
            , []
            )

        Prepared id (Ok s) ->
            ( setStatus id (always s) model, Cmd.none, [] )

        Prepared id (Err _) ->
            ( setStatus id (\s -> { s | state = "failed", reason = "internal" }) model, Cmd.none, [] )

        GotPhotographers (Ok list) ->
            ( { model | photographers = merge list model.photographers, photographersFailed = False }, Cmd.none, [] )

        GotPhotographers (Err _) ->
            ( { model | photographersFailed = True }, Cmd.none, [] )

        OpenPhotographer photoId existing ->
            ( { model
                | dialog =
                    Just
                        { forPhoto = photoId
                        , editing = Maybe.map .id existing
                        , name = existing |> Maybe.map .name |> Maybe.withDefault ""
                        , url = existing |> Maybe.map .url |> Maybe.withDefault ""
                        , saving = False
                        , error = Nothing
                        }
                , selectKey = model.selectKey + 1
              }
            , if model.photographersFailed then
                Api.getPhotographers GotPhotographers

              else
                Cmd.none
            , []
            )

        DialogName s ->
            ( mapDialog (\d -> { d | name = s }) model, Cmd.none, [] )

        DialogUrl s ->
            ( mapDialog (\d -> { d | url = s }) model, Cmd.none, [] )

        SavePhotographer ->
            case model.dialog of
                Just d ->
                    let
                        body =
                            { name = String.trim d.name, url = String.trim d.url }
                    in
                    ( mapDialog (\x -> { x | saving = True, error = Nothing }) model
                    , case d.editing of
                        Just id ->
                            Api.updatePhotographer id body SavedPhotographer

                        Nothing ->
                            Api.createPhotographer body SavedPhotographer
                    , []
                    )

                Nothing ->
                    ( model, Cmd.none, [] )

        SavedPhotographer (Ok ph) ->
            case model.dialog of
                Just d ->
                    ( { model
                        | dialog = Nothing
                        , photographers = merge [ ph ] model.photographers
                        , announce =
                            if d.editing == Nothing then
                                ph.name ++ " er lagt til som fotograf."

                            else
                                ph.name ++ " er lagret. Endringen gjelder alle bildene til denne fotografen."
                      }
                    , focusSelect d.forPhoto
                    , if d.editing == Nothing then
                        [ Credit d.forPhoto (Just ph.id) ]

                      else
                        []
                    )

                Nothing ->
                    ( model, Cmd.none, [] )

        SavedPhotographer (Err e) ->
            ( mapDialog (\d -> { d | saving = False, error = Just e }) model, Cmd.none, [] )

        UseExisting ph ->
            case model.dialog of
                Just d ->
                    ( { model | dialog = Nothing, photographers = merge [ ph ] model.photographers }
                    , focusSelect d.forPhoto
                    , [ Credit d.forPhoto (Just ph.id) ]
                    )

                Nothing ->
                    ( model, Cmd.none, [] )

        CloseDialog ->
            ( { model | dialog = Nothing }
            , model.dialog |> Maybe.map (.forPhoto >> focusSelect) |> Maybe.withDefault Cmd.none
            , []
            )

        Focused ->
            ( model, Cmd.none, [] )


name : Photo -> String
name p =
    if String.trim p.alt == "" then
        "Bildet"

    else
        "Bildet «" ++ p.alt ++ "»"


focusSelect : Int -> Cmd Msg
focusSelect photoId =
    Task.attempt (\_ -> Focused) (Dom.focus (selectId photoId))


selectId : Int -> String
selectId photoId =
    "fotograf-" ++ String.fromInt photoId


merge : List Photographer -> List Photographer -> List Photographer
merge new old =
    List.foldl
        (\ph acc ->
            if List.any (\x -> x.id == ph.id) acc then
                List.map
                    (\x ->
                        if x.id == ph.id then
                            ph

                        else
                            x
                    )
                    acc

            else
                acc ++ [ ph ]
        )
        old
        new
        |> List.sortBy (.name >> String.toLower)


mapUpload : Int -> (Upload -> Upload) -> Model -> Model
mapUpload key f model =
    { model
        | uploads =
            List.map
                (\u ->
                    if u.key == key then
                        f u

                    else
                        u
                )
                model.uploads
    }


mapDialog : (Dialog -> Dialog) -> Model -> Model
mapDialog f model =
    { model | dialog = Maybe.map f model.dialog }


setStatus : Int -> (PhotoStatus -> PhotoStatus) -> Model -> Model
setStatus id f model =
    { model | photos = Dict.update id (Maybe.map (\p -> { p | status = f p.status })) model.photos }


subscriptions : Model -> Sub Msg
subscriptions model =
    Sub.batch
        ((if List.isEmpty (pendingIds model) || model.polling then
            Sub.none

          else
            Time.every 2000 (\_ -> Poll)
         )
            :: List.map (\u -> Http.track (tracker u.key) (Progress u.key)) (List.filter (\u -> u.state == Sending) model.uploads)
        )



-- VIEW


uploadError : Api.Error -> String
uploadError e =
    case e of
        Api.Problems (p :: _) ->
            Text.uploadFailure p.code

        Api.Network ->
            "Mistet kontakten med serveren. Trykk «Prøv igjen», så sendes bildet på nytt."

        _ ->
            Text.uploadFailure ""


{-| One row per upload in flight or failed: what is happening, and what she
can do about it.
-}
uploadRow : (Msg -> msg) -> Upload -> Html msg
uploadRow toMsg u =
    let
        pct =
            if u.size > 0 then
                u.sent * 100 // u.size

            else
                0
    in
    div [ class "flex flex-col gap-2 p-3 rounded-lg border border-brown bg-white", A.attribute "data-upload" (File.name u.file) ]
        [ p [ class "font-bold break-all" ] [ text (File.name u.file) ]
        , case u.state of
            Sending ->
                div [ class "flex flex-col gap-2" ]
                    [ Ui.progress ("Laster opp … " ++ String.fromInt pct ++ " %") u.sent (max 1 u.size)
                    , div [] [ Ui.secondaryButton [] (toMsg (Cancel u.key)) "Avbryt" ]
                    ]

            Checking ->
                Ui.spinnerText "Sjekker bildet …"

            Failed e ->
                div [ class "flex flex-col gap-2" ]
                    [ Ui.errorText ("opplasting-" ++ String.fromInt u.key) [ uploadError e ]
                    , div [ class "flex flex-wrap gap-2" ]
                        [ case e of
                            Api.Problems _ ->
                                text ""

                            _ ->
                                Ui.primaryButton [] (toMsg (Retry u.key)) "Prøv igjen"
                        , Ui.secondaryButton [] (toMsg (Dismiss u.key)) "Fjern"
                        ]
                    ]
        ]


{-| The photo's own state once it is on the server.
-}
statusView : (Msg -> msg) -> Photo -> Html msg
statusView toMsg photo =
    let
        s =
            photo.status
    in
    div [ class "flex flex-col gap-1", A.attribute "data-photo-status" s.state ]
        (case s.state of
            "ready" ->
                [ p [ class "flex items-center gap-2 text-mork-eukalyptus font-bold" ]
                    [ span [ A.attribute "aria-hidden" "true" ] [ text "✓" ], text "Bildet er klart" ]
                ]

            "failed" ->
                [ Ui.errorText ("bildefeil-" ++ String.fromInt photo.id) [ Text.photoFailure s.reason ]
                , if s.reason == "damaged" || s.reason == "too_small" then
                    text ""

                  else
                    div [] [ Ui.secondaryButton [] (toMsg (Prepare photo.id)) "Prøv igjen" ]
                ]

            _ ->
                [ if s.total > 0 then
                    Ui.progress ("Klargjør bildet … " ++ String.fromInt s.done ++ " av " ++ String.fromInt s.total) s.done s.total

                  else
                    Ui.spinnerText "Klargjør bildet …"
                , Ui.help [] "Du kan fortsette å skrive mens bildet gjøres klart."
                ]
        )


{-| The photo, or a calm placeholder until the server has made its preview.
-}
image : List (Html.Attribute msg) -> Photo -> Html msg
image attrs photo =
    if photo.status.previewReady then
        img (A.src photo.thumbUrl :: A.alt photo.alt :: class "object-cover bg-kalk/40" :: attrs) []

    else
        div (class "flex items-center justify-center bg-kalk/40 text-center text-sm p-2" :: attrs)
            [ text
                (if photo.status.state == "failed" then
                    "Ingen forhåndsvisning"

                 else
                    "Bildet klargjøres …"
                )
            ]


{-| "Foto: name" for a photo's photographer, or "" for none.
-}
creditText : Model -> Maybe Int -> String
creditText model id =
    case photographer model id of
        Just ph ->
            "Foto: " ++ ph.name

        Nothing ->
            ""


photographer : Model -> Maybe Int -> Maybe Photographer
photographer model id =
    id |> Maybe.andThen (\i -> List.filter (\ph -> ph.id == i) model.photographers |> List.head)


{-| The photographer picker for one photo: a choice of the known
photographers, "new" in the same list, and a button to correct the chosen one.
-}
photographerField : { photoId : Int, selected : Maybe Int, onSelect : Maybe Int -> msg, toMsg : Msg -> msg } -> Model -> Html msg
photographerField c model =
    let
        id =
            selectId c.photoId

        chosen =
            photographer model c.selected

        choose v =
            if v == "ny" then
                c.toMsg (OpenPhotographer c.photoId Nothing)

            else
                c.onSelect (String.toInt v)
    in
    div [ class "flex flex-col gap-1.5" ]
        [ label [ A.for id, class "font-bold text-lg" ] [ text "Fotograf (valgfritt)" ]
        , div [ class "flex flex-wrap gap-2 items-center" ]
            [ Keyed.node "div"
                [ class "flex-1 min-w-48" ]
                [ ( String.fromInt model.selectKey
                  , Html.select
                        [ A.id id
                        , onInput choose
                        , class "w-full min-h-12 px-3 rounded-lg border-2 border-brown-text bg-white text-sort text-lg focus-visible:outline-3 focus-visible:outline-offset-1 focus-visible:outline-sort"
                        ]
                        (option [ A.value "", A.selected (c.selected == Nothing) ] [ text "Ingen fotograf" ]
                            :: List.map
                                (\ph -> option [ A.value (String.fromInt ph.id), A.selected (c.selected == Just ph.id) ] [ text ph.name ])
                                model.photographers
                            ++ [ option [ A.value "ny" ] [ text "Ny fotograf …" ] ]
                        )
                  )
                ]
            , case chosen of
                Just ph ->
                    Ui.quietButton [ A.attribute "aria-label" ("Rett navn eller lenke for " ++ ph.name) ] (c.toMsg (OpenPhotographer c.photoId (Just ph))) "Rett navn eller lenke"

                Nothing ->
                    text ""
            ]
        , case chosen of
            Just ph ->
                Ui.help []
                    (if ph.url == "" then
                        "Vises som «Foto: " ++ ph.name ++ "»."

                     else
                        "Vises som «Foto: " ++ ph.name ++ "», med lenke til " ++ ph.url ++ "."
                    )

            Nothing ->
                text ""
        , if model.photographersFailed then
            Ui.help [] "Klarte ikke å hente listen over fotografer. Velg «Ny fotograf …» for å prøve igjen."

          else
            text ""
        ]


dialog : (Msg -> msg) -> Model -> Html msg
dialog toMsg model =
    case model.dialog of
        Nothing ->
            text ""

        Just d ->
            let
                problems =
                    case d.error of
                        Just (Api.Problems ps) ->
                            ps

                        _ ->
                            []

                errorsFor f =
                    problems |> List.filter (\pr -> pr.field == Just f) |> List.map Text.problem
            in
            Html.map toMsg <|
                Ui.modal
                    { id = "fotograf-dialog"
                    , title =
                        if d.editing == Nothing then
                            "Ny fotograf"

                        else
                            "Rett fotografen"
                    , onClose = CloseDialog
                    , wide = False
                    }
                    [ if d.editing == Nothing then
                        Ui.help [] "Fotografen kan brukes igjen på andre bilder."

                      else
                        Ui.help [] "Endringen gjelder alle bildene til denne fotografen."
                    , Ui.field
                        { id = "fotograf-navn"
                        , label = "Navn"
                        , help = ""
                        , errors = errorsFor "name"
                        , control = Ui.input [ A.value d.name, onInput DialogName, A.autofocus True, A.attribute "autocomplete" "off" ]
                        }
                    , Ui.field
                        { id = "fotograf-lenke"
                        , label = "Lenke til fotografen (valgfritt)"
                        , help = "For eksempel nettsiden eller Instagram-siden. Lim inn hele adressen."
                        , errors = errorsFor "url"
                        , control = Ui.input [ A.value d.url, onInput DialogUrl, A.type_ "url", A.attribute "inputmode" "url" ]
                        }
                    , case d.error of
                        Just (Api.NameTaken existing) ->
                            Ui.notice "error"
                                [ p [] [ text ("Det finnes allerede en fotograf som heter «" ++ existing.name ++ "».") ]
                                , div [] [ Ui.primaryButton [] (UseExisting existing) ("Bruk «" ++ existing.name ++ "»") ]
                                ]

                        Just (Api.Problems _) ->
                            text ""

                        Just e ->
                            Ui.notice "error" [ p [] [ text (Text.error e) ] ]

                        Nothing ->
                            text ""
                    , div [ class "flex flex-wrap gap-3 justify-end" ]
                        [ Ui.secondaryButton [] CloseDialog "Avbryt"
                        , Ui.primaryButton [ A.disabled d.saving ]
                            SavePhotographer
                            (if d.saving then
                                "Lagrer …"

                             else
                                "Lagre fotografen"
                            )
                        ]
                    ]


{-| A one-line summary over several uploads and photos, for the gallery.
-}
busySummary : List Photo -> Model -> String
busySummary photos model =
    let
        count f =
            List.length (List.filter f model.uploads)

        sending =
            count (\u -> u.state == Sending || u.state == Checking)

        failedUploads =
            count
                (\u ->
                    case u.state of
                        Failed _ ->
                            True

                        _ ->
                            False
                )

        preparing =
            List.length (List.filter isPending photos)

        failed =
            List.length (List.filter (\ph -> ph.status.state == "failed") photos)

        part n one many =
            if n == 0 then
                []

            else if n == 1 then
                [ one ]

            else
                [ String.fromInt n ++ " " ++ many ]
    in
    String.join ", "
        (part sending "1 bilde lastes opp" "bilder lastes opp"
            ++ part preparing "1 bilde klargjøres" "bilder klargjøres"
            ++ part (failed + failedUploads) "1 bilde trenger hjelp (se under)" "bilder trenger hjelp (se under)"
        )


liveRegion : Model -> Html msg
liveRegion model =
    Ui.liveRegion model.announce
