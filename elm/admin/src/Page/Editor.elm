module Page.Editor exposing (Model, Msg, Out(..), eventId, init, isDirty, subscriptions, update, view)

{-| "Endre arrangement": the form, the crop dialog and the live preview. The
preview is the server's own rendering of the unsaved form, so what she sees
is what the site will show.
-}

import Api exposing (Event, Photo, PhotoRef, Problem, Rect)
import Browser.Dom as Dom
import Crop
import Dict exposing (Dict)
import Form exposing (Form)
import Html exposing (Html, a, div, iframe, img, label, li, node, p, span, text, ul)
import Html.Attributes as A exposing (attribute, class, style)
import Html.Events exposing (on, onClick, onInput)
import Html.Keyed as Keyed
import Json.Decode as D
import Json.Encode as E
import Photos
import Process
import Route
import Task
import Text
import Ui
import Venue


type alias Model =
    { id : Maybe Int
    , event : Maybe Event
    , loading : Loading
    , saved : Form
    , form : Form

    -- The body textarea is seeded once per load (see view); a new key gives
    -- a fresh one when the text is replaced from outside.
    , bodyKey : Int
    , photos : Photos.Model
    , problems : List Problem
    , problemsFor : Action
    , busy : Maybe Action
    , sending : Form
    , result : Maybe Action
    , failure : Maybe ( Action, Api.Error )
    , conflict : Maybe ( Action, Event )
    , crop : Maybe CropDialog
    , preview : Preview
    , announce : String
    }


type Loading
    = Loading
    | LoadFailed Api.Error
    | Ready


type Action
    = SaveDraft
    | SaveLive
    | Publish
    | Unpublish


type alias CropDialog =
    { photoId : Int
    , kind : String
    , rects : Dict String Rect
    , saving : Bool
    , error : Maybe Api.Error
    }


type alias Preview =
    { device : String
    , view : String
    , html : Maybe String
    , waiting : Bool
    , failed : Bool
    , seq : Int
    }


type Out
    = Created Int


init : Maybe Int -> ( Model, Cmd Msg )
init id =
    let
        ( photos, photosCmd ) =
            Photos.init

        model =
            { id = id
            , event = Nothing
            , loading =
                if id == Nothing then
                    Ready

                else
                    Loading
            , saved = Form.empty
            , form = Form.empty
            , bodyKey = 0
            , photos = photos
            , problems = []
            , problemsFor = SaveDraft
            , busy = Nothing
            , sending = Form.empty
            , result = Nothing
            , failure = Nothing
            , conflict = Nothing
            , crop = Nothing
            , preview = { device = "mobil", view = "page", html = Nothing, waiting = False, failed = False, seq = 0 }
            , announce = ""
            }
    in
    case id of
        Just i ->
            ( model, Cmd.batch [ Api.getEvent i Loaded, Cmd.map PhotosMsg photosCmd ] )

        Nothing ->
            schedulePreview ( model, Cmd.map PhotosMsg photosCmd )


eventId : Model -> Maybe Int
eventId =
    .id


{-| Unsaved typing, or a photo still on its way up.
-}
isDirty : Model -> Bool
isDirty model =
    model.form /= model.saved || List.any (\_ -> True) model.photos.uploads


status : Model -> String
status model =
    model.event |> Maybe.map .status |> Maybe.withDefault "draft"



-- UPDATE


type Field
    = Title
    | Slug
    | Start
    | StartTime
    | End
    | EndTime
    | Summary
    | Body
    | PriceText
    | TicketUrl
    | MenuLabel


type Msg
    = Loaded (Result Api.Error Event)
    | RetryLoad
    | Set Field String
    | SetMenu Bool
    | AddDate
    | SetDate Int String
    | RemoveDate Int
    | SetAlt Int String
    | SetPhotographer Int (Maybe Int)
    | RemoveMain
    | RemoveExtra Int
    | MoveExtra Int Int
    | PhotosMsg Photos.Msg
    | Run Action
    | Saved Action (Result Api.Error Event)
    | Done Action (Result Api.Error Event)
    | KeepMine
    | TakeTheirs
    | FocusField String
    | OpenCrop
    | CropKind String
    | GotCrop String Rect
    | CropNudge Int Int
    | CropResize Float
    | SaveCrop
    | CropSaved (Result Api.Error Photo)
    | CloseCrop
    | PreviewDue Int
    | GotPreview Int (Result Api.Error String)
    | SetDevice String
    | SetView String
    | RetryPreview
    | ShowPreview
    | Ignored


update : Msg -> Model -> ( Model, Cmd Msg, Maybe Out )
update msg model =
    case msg of
        Loaded (Ok e) ->
            schedulePreview ( load e { model | loading = Ready }, Cmd.none ) |> noOut

        Loaded (Err e) ->
            ( { model | loading = LoadFailed e }, Cmd.none, Nothing )

        RetryLoad ->
            ( { model | loading = Loading }, model.id |> Maybe.map (\i -> Api.getEvent i Loaded) |> Maybe.withDefault Cmd.none, Nothing )

        Set f s ->
            edit (setField f s) model

        SetMenu b ->
            edit (\form -> { form | showInMenu = b }) model

        AddDate ->
            let
                ( m, cmd, out ) =
                    edit (\form -> { form | extraDates = form.extraDates ++ [ "" ] }) model
            in
            ( m, Cmd.batch [ cmd, focus ("ekstra-dato-" ++ String.fromInt (List.length model.form.extraDates)) ], out )

        SetDate i s ->
            edit (\form -> { form | extraDates = Form.setAt i s form.extraDates }) model

        RemoveDate i ->
            let
                ( m, cmd, out ) =
                    edit (\form -> { form | extraDates = Form.removeAt i form.extraDates }) model
            in
            ( m, Cmd.batch [ cmd, focus "legg-til-dato" ], out )

        SetAlt id s ->
            edit (mapRef id (\r -> { r | alt = s })) model

        SetPhotographer id ph ->
            edit (mapRef id (\r -> { r | photographerId = ph })) model

        RemoveMain ->
            let
                ( m, cmd, out ) =
                    edit (\form -> { form | mainPhoto = Nothing }) model
            in
            ( { m | announce = "Bildet er tatt bort fra arrangementet." }, Cmd.batch [ cmd, focus "hovedbilde" ], out )

        RemoveExtra i ->
            let
                ( m, cmd, out ) =
                    edit (\form -> { form | extraPhotos = Form.removeAt i form.extraPhotos }) model
            in
            ( { m | announce = "Bildet er tatt bort." }, Cmd.batch [ cmd, focus "flere-bilder" ], out )

        MoveExtra i d ->
            let
                ( m, cmd, out ) =
                    edit (\form -> { form | extraPhotos = Form.move i d form.extraPhotos }) model
            in
            ( { m | announce = Text.movedTo (clamp 1 (List.length model.form.extraPhotos) (i + d + 1)) }
            , Cmd.batch [ cmd, focus (Ui.moveId (i + d) d) ]
            , out
            )

        PhotosMsg pm ->
            let
                ( photos, cmd, outs ) =
                    Photos.update pm model.photos
            in
            List.foldl applyPhotosOut ( { model | photos = photos }, Cmd.map PhotosMsg cmd, Nothing ) outs

        Run action ->
            run action { model | result = Nothing, failure = Nothing, conflict = Nothing }

        Saved action (Ok e) ->
            let
                m =
                    afterSave e model
            in
            case action of
                Publish ->
                    ( m, Api.publish e.id e.updatedAt (Done action), created model e )

                Unpublish ->
                    ( m, Api.unpublish e.id e.updatedAt (Done action), created model e )

                _ ->
                    ( finish action m, Cmd.none ) |> schedulePreview |> withOut (created model e)

        Saved action (Err e) ->
            failed action e model

        Done action (Ok e) ->
            ( finish action (afterSave e model), Cmd.none ) |> schedulePreview |> noOut

        Done action (Err e) ->
            failed action e model

        KeepMine ->
            case model.conflict of
                Just ( action, current ) ->
                    run action { model | event = Just current, conflict = Nothing }

                Nothing ->
                    ( model, Cmd.none, Nothing )

        TakeTheirs ->
            case model.conflict of
                Just ( _, current ) ->
                    schedulePreview ( load current { model | conflict = Nothing, announce = "Den nyeste versjonen er hentet." }, Cmd.none ) |> noOut

                Nothing ->
                    ( model, Cmd.none, Nothing )

        FocusField id ->
            ( model, focus id, Nothing )

        OpenCrop ->
            case model.form.mainPhoto |> Maybe.andThen (\r -> Photos.get r.id model.photos) of
                Just photo ->
                    ( { model | crop = Just { photoId = photo.id, kind = "card", rects = photo.crops, saving = False, error = Nothing } }, Cmd.none, Nothing )

                Nothing ->
                    ( model, Cmd.none, Nothing )

        CropKind k ->
            ( mapCrop (\c -> { c | kind = k }) model, Cmd.none, Nothing )

        GotCrop k r ->
            ( mapCrop (\c -> { c | rects = Dict.insert k r c.rects }) model, Cmd.none, Nothing )

        CropNudge dx dy ->
            ( mapCropRect (\_ size -> Crop.nudge size dx dy) model, Cmd.none, Nothing )

        CropResize f ->
            ( mapCropRect (\kind size -> Crop.resize kind size f) model, Cmd.none, Nothing )

        SaveCrop ->
            case model.crop of
                Just c ->
                    ( mapCrop (\x -> { x | saving = True, error = Nothing }) model, Api.saveCrops c.photoId c.rects CropSaved, Nothing )

                Nothing ->
                    ( model, Cmd.none, Nothing )

        CropSaved (Ok photo) ->
            schedulePreview
                ( { model | crop = Nothing, photos = Photos.add [ photo ] model.photos, announce = "Beskjæringen er lagret." }
                , focus "beskjaer"
                )
                |> noOut

        CropSaved (Err e) ->
            ( mapCrop (\c -> { c | saving = False, error = Just e }) model, Cmd.none, Nothing )

        CloseCrop ->
            ( { model | crop = Nothing }, focus "beskjaer", Nothing )

        PreviewDue seq ->
            if seq == model.preview.seq then
                ( model, requestPreview model, Nothing )

            else
                ( model, Cmd.none, Nothing )

        GotPreview seq result ->
            if seq /= model.preview.seq then
                ( model, Cmd.none, Nothing )

            else
                let
                    pv =
                        model.preview
                in
                case result of
                    Ok html ->
                        ( { model | preview = { pv | html = Just html, waiting = False, failed = False } }, Cmd.none, Nothing )

                    Err _ ->
                        ( { model | preview = { pv | waiting = False, failed = True } }, Cmd.none, Nothing )

        SetDevice d ->
            let
                pv =
                    model.preview
            in
            ( { model | preview = { pv | device = d } }, Cmd.none, Nothing )

        SetView v ->
            let
                pv =
                    model.preview
            in
            schedulePreview ( { model | preview = { pv | view = v, html = Nothing } }, Cmd.none ) |> noOut

        RetryPreview ->
            schedulePreview ( model, Cmd.none ) |> noOut

        ShowPreview ->
            ( model
            , Dom.getElement "forhandsvisning"
                |> Task.andThen (\el -> Dom.setViewport 0 el.element.y)
                |> Task.attempt (\_ -> Ignored)
            , Nothing
            )

        Ignored ->
            ( model, Cmd.none, Nothing )


noOut : ( Model, Cmd Msg ) -> ( Model, Cmd Msg, Maybe Out )
noOut ( m, c ) =
    ( m, c, Nothing )


withOut : Maybe Out -> ( Model, Cmd Msg ) -> ( Model, Cmd Msg, Maybe Out )
withOut out ( m, c ) =
    ( m, c, out )


created : Model -> Event -> Maybe Out
created model e =
    if model.id == Nothing then
        Just (Created e.id)

    else
        Nothing


{-| A form change: keep it, and ask for a fresh preview once typing pauses.
-}
edit : (Form -> Form) -> Model -> ( Model, Cmd Msg, Maybe Out )
edit f model =
    schedulePreview ( { model | form = f model.form, result = Nothing }, Cmd.none ) |> noOut


schedulePreview : ( Model, Cmd Msg ) -> ( Model, Cmd Msg )
schedulePreview ( model, cmd ) =
    let
        pv =
            model.preview

        seq =
            pv.seq + 1
    in
    ( { model | preview = { pv | seq = seq, waiting = True } }
    , Cmd.batch [ cmd, Process.sleep 600 |> Task.perform (\_ -> PreviewDue seq) ]
    )


requestPreview : Model -> Cmd Msg
requestPreview model =
    Api.preview model.preview.view model.id (Form.toInput "" model.form) (GotPreview model.preview.seq)


load : Event -> Model -> Model
load e model =
    let
        form =
            Form.fromEvent e
    in
    { model
        | id = Just e.id
        , event = Just e
        , saved = form
        , form = form
        , bodyKey = model.bodyKey + 1
        , photos = Photos.add (List.filterMap identity [ e.mainPhoto ] ++ e.extraPhotos) model.photos
        , problems = []
    }


afterSave : Event -> Model -> Model
afterSave e model =
    let
        stored =
            Form.fromEvent e

        form =
            Form.afterSave { sent = model.sending, current = model.form, stored = stored }
    in
    { model
        | id = Just e.id
        , event = Just e
        , saved = stored
        , form = form
        , bodyKey =
            if form.body /= model.form.body then
                model.bodyKey + 1

            else
                model.bodyKey
        , photos = Photos.add (List.filterMap identity [ e.mainPhoto ] ++ e.extraPhotos) model.photos
    }


{-| Saves first when there is anything to save, then publishes or hides.
-}
run : Action -> Model -> ( Model, Cmd Msg, Maybe Out )
run action model =
    let
        updatedAt =
            model.event |> Maybe.map .updatedAt |> Maybe.withDefault ""

        input =
            Form.toInput updatedAt model.form

        busy =
            { model | busy = Just action, sending = model.form, announce = busyText action }
    in
    case ( model.id, action, model.form == model.saved ) of
        ( Nothing, _, _ ) ->
            ( busy, Api.createEvent input (Saved action), Nothing )

        ( Just id, Publish, True ) ->
            ( busy, Api.publish id updatedAt (Done action), Nothing )

        ( Just id, Unpublish, True ) ->
            ( busy, Api.unpublish id updatedAt (Done action), Nothing )

        ( Just id, _, _ ) ->
            ( busy, Api.updateEvent id input (Saved action), Nothing )


busyText : Action -> String
busyText action =
    case action of
        Publish ->
            "Publiserer …"

        Unpublish ->
            "Gjør om til kladd …"

        _ ->
            "Lagrer …"


finish : Action -> Model -> Model
finish action model =
    { model
        | busy = Nothing
        , problems = []
        , result = Just action
        , announce = doneText action model
    }


doneText : Action -> Model -> String
doneText action model =
    case action of
        SaveDraft ->
            "Kladden er lagret. Den vises ikke på nettsiden."

        SaveLive ->
            "Endringene er lagret og vises på nettsiden."

        Publish ->
            "Publisert! Arrangementet er nå på nettsiden."
                ++ (if photosPending model then
                        " Noen bilder klargjøres fortsatt. De kommer på plass på nettsiden av seg selv om litt."

                    else
                        ""
                   )

        Unpublish ->
            "Arrangementet er gjort om til kladd og vises ikke lenger på nettsiden."


failed : Action -> Api.Error -> Model -> ( Model, Cmd Msg, Maybe Out )
failed action e model =
    let
        m =
            { model | busy = Nothing, announce = "" }
    in
    case e of
        Api.Problems ps ->
            ( { m | problems = ps, problemsFor = action }, focus "feil-oppsummering", Nothing )

        Api.Conflict current ->
            ( { m | conflict = Just ( action, current ) }, Cmd.none, Nothing )

        _ ->
            ( { m | failure = Just ( action, e ) }, focus "lagre-feil", Nothing )


applyPhotosOut : Photos.Out -> ( Model, Cmd Msg, Maybe Out ) -> ( Model, Cmd Msg, Maybe Out )
applyPhotosOut out ( model, cmd, o ) =
    let
        ref p =
            { id = p.id, alt = p.alt, photographerId = Maybe.map .id p.photographer }

        ( m, c, _ ) =
            case out of
                Photos.Added Photos.Main p ->
                    edit (\f -> { f | mainPhoto = Just (ref p) }) model

                Photos.Added _ p ->
                    edit
                        (\f ->
                            if List.any (\r -> r.id == p.id) f.extraPhotos then
                                f

                            else
                                { f | extraPhotos = f.extraPhotos ++ [ ref p ] }
                        )
                        model

                Photos.Credit id ph ->
                    edit (mapRef id (\r -> { r | photographerId = ph })) model
    in
    ( m, Cmd.batch [ cmd, c ], o )


mapRef : Int -> (PhotoRef -> PhotoRef) -> Form -> Form
mapRef id f form =
    let
        g r =
            if r.id == id then
                f r

            else
                r
    in
    { form | mainPhoto = Maybe.map g form.mainPhoto, extraPhotos = List.map g form.extraPhotos }


setField : Field -> String -> Form -> Form
setField f s form =
    case f of
        Title ->
            { form | title = s }

        Slug ->
            { form | slug = s }

        Start ->
            { form | start = s }

        StartTime ->
            { form | startTime = s }

        End ->
            { form | end = s }

        EndTime ->
            { form | endTime = s }

        Summary ->
            { form | summary = s }

        Body ->
            { form | body = s }

        PriceText ->
            { form | priceText = s }

        TicketUrl ->
            { form | ticketUrl = s }

        MenuLabel ->
            { form | menuLabel = s }


mapCrop : (CropDialog -> CropDialog) -> Model -> Model
mapCrop f model =
    { model | crop = Maybe.map f model.crop }


mapCropRect : (Crop.Kind -> Crop.Size -> Rect -> Rect) -> Model -> Model
mapCropRect f model =
    case model.crop of
        Just c ->
            case ( Photos.get c.photoId model.photos, Dict.get c.kind c.rects ) of
                ( Just photo, Just r ) ->
                    { model | crop = Just { c | rects = Dict.insert c.kind (f (Crop.find photo.quality c.kind) { width = photo.width, height = photo.height } r) c.rects } }

                _ ->
                    model

        Nothing ->
            model


focus : String -> Cmd Msg
focus =
    Ui.focus Ignored


photosPending : Model -> Bool
photosPending model =
    formPhotos model |> List.any Photos.isPending


formPhotos : Model -> List Photo
formPhotos model =
    (List.filterMap identity [ model.form.mainPhoto ] ++ model.form.extraPhotos)
        |> List.filterMap (\r -> Photos.get r.id model.photos)


subscriptions : Model -> Sub Msg
subscriptions model =
    Sub.map PhotosMsg (Photos.subscriptions model.photos)



-- VIEW


view : Model -> Html Msg
view model =
    case model.loading of
        Loading ->
            Ui.spinnerText "Henter arrangementet …"

        LoadFailed e ->
            Ui.notice "error"
                [ p [ attribute "role" "alert" ] [ text ("Klarte ikke å hente arrangementet. " ++ Text.error e) ]
                , div [ class "flex flex-wrap gap-3" ]
                    [ Ui.primaryButton [] RetryLoad "Prøv igjen"
                    , a [ Route.href Route.Overview, class "inline-flex items-center min-h-12 font-bold underline" ] [ text "Tilbake til arrangementene" ]
                    ]
                ]

        Ready ->
            div [ class "flex flex-col gap-6" ]
                [ actionBar model
                , Ui.liveRegion model.announce
                , Photos.liveRegion model.photos
                , div [ class "grid gap-8 grid-cols-[minmax(0,1fr)] lg:grid-cols-[minmax(0,1fr)_minmax(0,30rem)] items-start" ]
                    [ div [ class "flex flex-col gap-5 min-w-0" ]
                        (div [ class "lg:hidden" ] [ Ui.secondaryButton [] ShowPreview "Se hvordan det blir" ]
                            :: messages model
                            ++ formView model
                        )
                    , previewView model
                    ]
                , mobileBar model
                , cropDialog model
                , conflictDialog model
                , Photos.dialog PhotosMsg model.photos
                ]


actionBar : Model -> Html Msg
actionBar model =
    div [ class "flex flex-wrap gap-3 items-center justify-between md:sticky md:top-0 z-10 bg-white/95 border border-brown rounded-xl p-3 md:p-4" ]
        [ div [ class "flex flex-wrap gap-3 items-center" ]
            [ a [ Route.href Route.Overview, class "inline-flex items-center gap-2 min-h-11 px-2 font-bold underline underline-offset-4 focus-visible:outline-3 focus-visible:outline-sort" ]
                [ span [ attribute "aria-hidden" "true" ] [ text "←" ], text "Arrangementer" ]
            , if status model == "published" then
                Ui.badge "published" "Publisert – vises på nettsiden"

              else
                Ui.badge "draft" "Kladd – vises ikke på nettsiden"
            , if isDirty model then
                span [ class "text-brown-text" ] [ text "Ikke lagret" ]

              else
                text ""
            ]
        , div [ class "hidden md:flex flex-wrap gap-3 items-center" ] (actionButtons model)
        ]


mobileBar : Model -> Html Msg
mobileBar model =
    div [ class "md:hidden sticky bottom-0 z-20 -mx-4 bg-white border-t border-brown p-3 grid grid-cols-2 gap-3" ]
        (actionButtons model)


actionButtons : Model -> List (Html Msg)
actionButtons model =
    let
        busy =
            model.busy /= Nothing

        label action idle =
            if model.busy == Just action then
                busyText action

            else
                idle
    in
    if status model == "published" then
        [ Ui.quietButton [ A.disabled busy ] (Run Unpublish) (label Unpublish "Gjør om til kladd")
        , Ui.primaryButton [ A.disabled busy ] (Run SaveLive) (label SaveLive "Lagre endringene")
        ]

    else
        [ Ui.secondaryButton [ A.disabled busy ] (Run SaveDraft) (label SaveDraft "Lagre kladd")
        , Ui.primaryButton [ A.disabled busy ] (Run Publish) (label Publish "Publiser")
        ]


messages : Model -> List (Html Msg)
messages model =
    [ case model.result of
        Just action ->
            Ui.notice "ok"
                [ p [ class "font-bold" ] [ text (doneText action model) ]
                , case ( action, model.event ) of
                    ( Publish, Just e ) ->
                        a [ A.href e.url, A.target "_blank", A.rel "noopener", class "underline font-bold min-h-11 inline-flex items-center" ] [ text "Se siden på nettsiden (åpnes i ny fane)" ]

                    _ ->
                        text ""
                ]

        Nothing ->
            text ""
    , case model.failure of
        Just ( action, e ) ->
            div [ A.id "lagre-feil", A.tabindex -1 ]
                [ Ui.notice "error"
                    [ p [ attribute "role" "alert" ] [ text (Text.error e) ]
                    , div [] [ Ui.primaryButton [] (Run action) "Prøv igjen" ]
                    ]
                ]

        Nothing ->
            text ""
    , if List.isEmpty model.problems then
        text ""

      else
        Ui.errorList "feil-oppsummering"
            (if model.problemsFor == Publish then
                "Dette mangler før arrangementet kan publiseres:"

             else
                "Dette må rettes før det kan lagres:"
            )
            (List.map (\pr -> ( Text.problem pr, Form.fieldId pr |> Maybe.map FocusField )) model.problems)
    , if photosPending model && model.result == Nothing then
        Ui.help [] "Noen bilder klargjøres fortsatt. Du kan likevel lagre og publisere; bildene kommer på plass av seg selv."

      else
        text ""
    ]


errs : Model -> String -> List String
errs model id =
    Form.errorsFor id model.problems


text_ : Model -> String -> String -> Field -> String -> String -> Html Msg
text_ model id lbl fld value helpText =
    Ui.field
        { id = id
        , label = lbl
        , help = helpText
        , errors = errs model id
        , control = Ui.input [ A.value value, onInput (Set fld) ]
        }


formView : Model -> List (Html Msg)
formView model =
    let
        f =
            model.form

        frozen =
            model.event |> Maybe.map .slugFrozen |> Maybe.withDefault False

        year =
            String.left 4 f.start
    in
    [ Ui.card []
        [ text_ model "tittel" "Tittel" Title f.title ""
        , if frozen then
            div [ class "flex flex-col gap-1" ]
                [ p [ class "font-bold text-lg" ] [ text "Adresse til siden" ]
                , p [ class "break-all" ] [ text ("aspargesgaarden.no" ++ (model.event |> Maybe.map .path |> Maybe.withDefault "")) ]
                , Ui.help [] "Adressen endres ikke etter at siden er publisert, så lenker som er delt fortsatt virker."
                ]

          else
            Ui.field
                { id = "adresse"
                , label = "Adresse til siden (valgfritt)"
                , help =
                    "aspargesgaarden.no/arrangementer/"
                        ++ (if year == "" then
                                "<år>"

                            else
                                year
                           )
                        ++ "/"
                        ++ (if f.slug == "" then
                                "…"

                            else
                                f.slug
                           )
                        ++ " – lages av tittelen hvis du lar feltet stå tomt. Kan ikke endres etter publisering."
                , errors = errs model "adresse"
                , control = Ui.input [ A.value f.slug, onInput (Set Slug), A.placeholder "Lages av tittelen", attribute "autocapitalize" "off", attribute "spellcheck" "false" ]
                }
        ]
    , Ui.card []
        [ Html.h2 [ class "font-display text-tre text-2xl" ] [ text "Når er det?" ]
        , div [ class "grid gap-4 sm:grid-cols-2" ]
            [ dateField model "dato" "Dato" Start f.start
            , timeField model "klokken" "Starter klokken (valgfritt)" StartTime f.startTime
            , dateField model "siste-dag" "Siste dag (valgfritt)" End f.end
            , timeField model "slutter" "Slutter klokken (valgfritt)" EndTime f.endTime
            ]
        , Ui.help [] "«Siste dag» er for arrangementer som varer flere dager på rad, som julemarkedet."
        , div [ A.id "flere-datoer", A.tabindex -1, class "flex flex-col gap-3" ]
            [ if List.isEmpty f.extraDates then
                text ""

              else
                p [ class "font-bold text-lg" ] [ text "Flere datoer" ]
            , ul [ class "flex flex-col gap-3" ]
                (List.indexedMap
                    (\i d ->
                        li [ class "flex flex-wrap gap-3 items-end" ]
                            [ div [ class "flex-1 min-w-48" ]
                                [ Ui.field
                                    { id = "ekstra-dato-" ++ String.fromInt i
                                    , label = "Dag " ++ String.fromInt (i + 2)
                                    , help = ""
                                    , errors = []
                                    , control = Ui.input [ A.type_ "date", A.value d, onInput (SetDate i) ]
                                    }
                                ]
                            , Ui.quietButton [ attribute "aria-label" ("Ta bort dag " ++ String.fromInt (i + 2)) ] (RemoveDate i) "Ta bort"
                            ]
                    )
                    f.extraDates
                )
            , Ui.errorText "flere-datoer-feil" (errs model "flere-datoer")
            , div []
                [ button_ [ A.id "legg-til-dato", class "inline-flex items-center min-h-11 px-4 rounded-lg border-2 border-dashed border-brown-text font-bold hover:bg-brown/20 focus-visible:outline-3 focus-visible:outline-offset-2 focus-visible:outline-sort" ]
                    AddDate
                    "+ Legg til en dag til (for kurs over flere dager)"
                ]
            , if List.isEmpty f.extraDates then
                text ""

              else
                Ui.help [] "Hver dag får samme klokkeslett som første dag."
            ]
        ]
    , mainPhotoView model
    , Ui.card []
        [ Ui.field
            { id = "kort-tekst"
            , label = "Kort tekst"
            , help = "Én eller to setninger. Vises i listen, på Google og når noen deler lenken på Facebook."
            , errors = errs model "kort-tekst"
            , control = Ui.textarea [ A.value f.summary, onInput (Set Summary), A.rows 3 ]
            }
        , p
            [ class "text-right"
            , class
                (if String.length f.summary > Form.summaryLimit then
                    "text-rust font-bold"

                 else
                    "text-brown-text"
                )
            ]
            [ text
                (String.fromInt (String.length f.summary)
                    ++ " av "
                    ++ String.fromInt Form.summaryLimit
                    ++ " tegn"
                    ++ (if String.length f.summary > Form.summaryLimit then
                            " – Google viser bare starten"

                        else
                            ""
                       )
                )
            ]
        ]
    , bodyView model
    , Ui.card []
        [ Html.h2 [ class "font-display text-tre text-2xl" ] [ text "Pris og påmelding (valgfritt)" ]
        , div [ class "grid gap-4 sm:grid-cols-2" ]
            [ Ui.field
                { id = "pris"
                , label = "Pris"
                , help = "For eksempel «250,- per person»."
                , errors = errs model "pris"
                , control = Ui.input [ A.value f.priceText, onInput (Set PriceText) ]
                }
            , Ui.field
                { id = "billettlenke"
                , label = "Lenke til billetter"
                , help = "Lim inn hele adressen, med https://."
                , errors = errs model "billettlenke"
                , control = Ui.input [ A.value f.ticketUrl, onInput (Set TicketUrl), A.type_ "url", attribute "inputmode" "url" ]
                }
            ]
        ]
    , extraPhotosView model
    , Ui.card []
        [ Ui.checkbox
            { id = "meny"
            , label = "Vis i menyen øverst på nettsiden"
            , help = "Brukes for julemarkedet. Arrangementet får sin egen lenke i menyen til det er over."
            , checked = f.showInMenu
            , onCheck = SetMenu
            }
        , if f.showInMenu then
            Ui.field
                { id = "menynavn"
                , label = "Navn i menyen (valgfritt)"
                , help = "Kort, for eksempel «Julemarked». Står det tomt, brukes tittelen."
                , errors = errs model "menynavn"
                , control = Ui.input [ A.value f.menuLabel, onInput (Set MenuLabel) ]
                }

          else
            text ""
        ]
    ]


button_ : List (Html.Attribute Msg) -> Msg -> String -> Html Msg
button_ attrs msg s =
    Html.button (A.type_ "button" :: onClick msg :: attrs) [ text s ]


dateField : Model -> String -> String -> Field -> String -> Html Msg
dateField model id lbl fld value =
    Ui.field
        { id = id, label = lbl, help = "", errors = errs model id, control = Ui.input [ A.type_ "date", A.value value, onInput (Set fld) ] }


timeField : Model -> String -> String -> Field -> String -> Html Msg
timeField model id lbl fld value =
    Ui.field
        { id = id, label = lbl, help = "", errors = errs model id, control = Ui.input [ A.type_ "time", A.value value, onInput (Set fld) ] }


bodyView : Model -> Html Msg
bodyView model =
    let
        tool name content attrs =
            node name
                (class "inline-flex items-center justify-center min-h-11 px-4 rounded-md border border-brown-text bg-brown/20 cursor-pointer select-none hover:bg-brown/40 focus-visible:outline-3 focus-visible:outline-offset-2 focus-visible:outline-sort" :: attrs)
                [ text content ]
    in
    Ui.card []
        [ label [ A.for "hele-teksten", class "font-bold text-lg" ] [ text "Hele teksten" ]
        , Ui.help [ A.id "hele-teksten-hjelp" ] "Marker tekst og trykk på en knapp for å gjøre den fet, lage overskrift eller liste. Forhåndsvisningen viser hvordan det blir."
        , node "markdown-toolbar"
            [ attribute "for" "hele-teksten", class "flex flex-wrap gap-2", attribute "aria-label" "Formatering" ]
            [ tool "md-bold" "Fet" [ class "font-bold" ]
            , tool "md-italic" "Kursiv" [ class "italic" ]
            , tool "md-header" "Overskrift" []
            , tool "md-unordered-list" "Liste" []
            , tool "md-link" "Lenke" []
            , tool "asp-md-insert" "Telefonlenke" [ attribute "text" Venue.phoneLink ]
            ]

        -- The DOM owns the text so toolbar edits and undo work; Elm seeds it
        -- through defaultValue and listens. A new key replaces the textarea.
        , Keyed.node "div"
            []
            [ ( String.fromInt model.bodyKey
              , Ui.textarea
                    [ A.id "hele-teksten"
                    , A.rows 12
                    , A.property "defaultValue" (E.string model.form.body)
                    , onInput (Set Body)
                    , attribute "aria-describedby" "hele-teksten-hjelp"
                    ]
                    []
              )
            ]
        , Ui.errorText "hele-teksten-feil" (errs model "hele-teksten")
        ]


mainPhotoView : Model -> Html Msg
mainPhotoView model =
    Ui.card [ A.id "bilde" ]
        [ Html.h2 [ class "font-display text-tre text-2xl" ] [ text "Bilde" ]
        , Ui.help [] "Vises i listen over arrangementer, øverst på siden og når noen deler lenken."
        , case model.form.mainPhoto |> Maybe.andThen (\r -> Photos.get r.id model.photos |> Maybe.map (Tuple.pair r)) of
            Just ( ref, photo ) ->
                div [ class "flex flex-wrap gap-5 items-start" ]
                    [ Photos.image [ class "w-36 h-44 rounded-lg" ] photo
                    , div [ class "flex-1 min-w-60 flex flex-col gap-4" ]
                        [ Photos.statusView PhotosMsg photo
                        , div [ class "flex flex-wrap gap-2" ]
                            [ Ui.secondaryButton [ A.id "hovedbilde" ] (PhotosMsg (Photos.pick Photos.Main False)) "Bytt bilde"
                            , Ui.secondaryButton [ A.id "beskjaer", A.disabled (not photo.status.previewReady) ] OpenCrop "Beskjær"
                            , Ui.quietButton [] RemoveMain "Ta bort bildet"
                            ]
                        , if not photo.status.previewReady then
                            Ui.help [] "Du kan beskjære bildet så snart det vises."

                          else
                            text ""
                        , smallPhotoNote photo
                        , altField model ref
                        , Photos.photographerField { photoId = ref.id, selected = ref.photographerId, onSelect = SetPhotographer ref.id, toMsg = PhotosMsg } model.photos
                        ]
                    ]

            Nothing ->
                div [ class "flex flex-col gap-2" ]
                    [ div [] [ Ui.primaryButton [ A.id "hovedbilde", attribute "aria-describedby" "hovedbilde-feil" ] (PhotosMsg (Photos.pick Photos.Main False)) "Velg bilde" ]
                    , Ui.errorText "hovedbilde-feil" (errs model "hovedbilde")
                    ]
        , Ui.errorText "hovedbilde-feil2"
            (if model.form.mainPhoto == Nothing then
                []

             else
                errs model "hovedbilde"
            )
        , div [ class "flex flex-col gap-2" ] (List.map (Photos.uploadRow PhotosMsg) (Photos.uploadsFor Photos.Main model.photos))
        ]


{-| How the main photo fares in each crop: what a kind it is too small for
gets instead, and which kinds will look soft, said once beside the photo.
The crop dialog says it again where she would look for it.
-}
smallPhotoNote : Photo -> Html Msg
smallPhotoNote photo =
    let
        size =
            { width = photo.width, height = photo.height }

        note kind =
            case ( Crop.usable kind size, kind.quality.soft ) of
                ( False, False ) ->
                    Just ( False, fallbackText kind )

                ( False, True ) ->
                    Just ( True, fallbackText kind ++ " " ++ Text.softWhole )

                ( True, True ) ->
                    Just ( True, Text.softKind kind.usedFor )

                ( True, False ) ->
                    Nothing

        notes =
            List.filterMap note (Crop.kinds photo.quality)

        soft =
            List.filter Tuple.first notes |> List.map Tuple.second

        calm =
            List.filter (not << Tuple.first) notes |> List.map Tuple.second
    in
    if List.isEmpty notes then
        text ""

    else
        div [ class "flex flex-col gap-2" ]
            (List.map (Ui.help []) calm
                ++ (if List.isEmpty soft then
                        []

                    else
                        [ Ui.verdict "soft" [ A.id "bilde-uskarpt" ] (soft ++ [ Text.askForOriginal ]) ]
                   )
            )


fallbackText : Crop.Kind -> String
fallbackText kind =
    Text.noCrop kind.key kind.quality.minWidth


altField : Model -> PhotoRef -> Html Msg
altField model ref =
    let
        id =
            "alt-" ++ String.fromInt ref.id
    in
    Ui.field
        { id = id
        , label = "Beskriv bildet"
        , help = "Hva viser bildet? Leses opp for blinde, og hjelper Google å forstå bildet."
        , errors = errs model id
        , control = Ui.input [ A.value ref.alt, onInput (SetAlt ref.id) ]
        }


extraPhotosView : Model -> Html Msg
extraPhotosView model =
    let
        n =
            List.length model.form.extraPhotos
    in
    Ui.card [ A.id "flere-bilder-seksjon" ]
        [ Html.h2 [ class "font-display text-tre text-2xl" ] [ text "Flere bilder (valgfritt)" ]
        , Ui.help [] "Vises nederst på siden til arrangementet, i rekkefølgen under."
        , ul [ class "flex flex-col gap-4" ]
            (List.indexedMap
                (\i ref ->
                    case Photos.get ref.id model.photos of
                        Just photo ->
                            li [ class "flex flex-wrap gap-4 items-start border-t border-brown pt-4 first:border-0 first:pt-0" ]
                                [ Photos.image [ class "w-28 h-28 rounded-lg" ] photo
                                , div [ class "flex-1 min-w-60 flex flex-col gap-3" ]
                                    [ p [ class "font-bold" ] [ text ("Bilde " ++ String.fromInt (i + 1) ++ " av " ++ String.fromInt n) ]
                                    , Photos.statusView PhotosMsg photo
                                    , Photos.sizeNote photo
                                    , altField model ref
                                    , Photos.photographerField { photoId = ref.id, selected = ref.photographerId, onSelect = SetPhotographer ref.id, toMsg = PhotosMsg } model.photos
                                    , div [ class "flex flex-wrap gap-2" ]
                                        [ Ui.secondaryButton [ A.id (Ui.moveId i -1), A.disabled (i == 0), attribute "aria-label" ("Flytt bilde " ++ String.fromInt (i + 1) ++ " tidligere") ] (MoveExtra i -1) "↑ Tidligere"
                                        , Ui.secondaryButton [ A.id (Ui.moveId i 1), A.disabled (i == n - 1), attribute "aria-label" ("Flytt bilde " ++ String.fromInt (i + 1) ++ " senere") ] (MoveExtra i 1) "↓ Senere"
                                        , Ui.quietButton [ attribute "aria-label" ("Ta bort bilde " ++ String.fromInt (i + 1)) ] (RemoveExtra i) "Ta bort"
                                        ]
                                    ]
                                ]

                        Nothing ->
                            text ""
                )
                model.form.extraPhotos
            )
        , div [ class "flex flex-col gap-2" ] (List.map (Photos.uploadRow PhotosMsg) (Photos.uploadsFor Photos.Extra model.photos))
        , Ui.errorText "flere-bilder-feil" (errs model "flere-bilder")
        , div [] [ Ui.secondaryButton [ A.id "flere-bilder" ] (PhotosMsg (Photos.pick Photos.Extra True)) "+ Legg til bilder" ]
        ]



-- PREVIEW


previewView : Model -> Html Msg
previewView model =
    let
        pv =
            model.preview

        toggle current value lbl msg =
            Html.button
                [ A.type_ "button"
                , onClick (msg value)
                , attribute "aria-pressed"
                    (if current == value then
                        "true"

                     else
                        "false"
                    )
                , class "min-h-11 px-4 font-bold focus-visible:outline-3 focus-visible:-outline-offset-4 focus-visible:outline-mose aria-pressed:bg-sort aria-pressed:text-white bg-white text-sort"
                ]
                [ text lbl ]

        ( w, h ) =
            if pv.device == "mobil" then
                ( 390, 740 )

            else
                ( 1280, 800 )
    in
    Html.section
        [ A.id "forhandsvisning"
        , attribute "aria-labelledby" "forhandsvisning-tittel"
        , class "bg-white border border-brown rounded-xl p-4 md:p-5 flex flex-col gap-4 lg:sticky lg:top-24"
        ]
        [ Html.h2 [ A.id "forhandsvisning-tittel", class "font-display text-tre text-3xl" ] [ text "Slik ser det ut" ]
        , div [ class "flex flex-wrap gap-3" ]
            [ div [ class "inline-flex border-2 border-sort rounded-lg overflow-hidden", attribute "role" "group", attribute "aria-label" "Skjerm" ]
                [ toggle pv.device "mobil" "Mobil" SetDevice, toggle pv.device "pc" "PC" SetDevice ]
            , div [ class "inline-flex border-2 border-sort rounded-lg overflow-hidden", attribute "role" "group", attribute "aria-label" "Side" ]
                [ toggle pv.view "page" "Egen side" SetView, toggle pv.view "list" "I listen" SetView ]
            ]
        , if pv.failed then
            Ui.notice "error"
                [ p [] [ text "Klarte ikke å lage forhåndsvisningen. Det du har skrevet er ikke borte." ]
                , div [] [ Ui.secondaryButton [] RetryPreview "Prøv igjen" ]
                ]

          else if pv.waiting then
            Ui.spinnerText "Oppdaterer …"

          else
            Ui.help [] "Oppdateres mens du skriver."
        , case pv.html of
            Just html ->
                node "asp-fit"
                    [ attribute "width" (String.fromInt w)
                    , attribute "height" (String.fromInt h)
                    , class "block w-full overflow-hidden bg-white"
                    , class
                        (if pv.device == "mobil" then
                            "mx-auto max-w-[406px] border-8 border-sort rounded-[2rem]"

                         else
                            "border-4 border-sort rounded-lg"
                        )
                    ]
                    [ iframe
                        [ A.srcdoc html
                        , attribute "sandbox" "allow-same-origin"
                        , A.title
                            ("Forhåndsvisning på "
                                ++ (if pv.device == "mobil" then
                                        "mobil"

                                    else
                                        "PC"
                                   )
                            )
                        , style "width" (String.fromInt w ++ "px")
                        , style "height" (String.fromInt h ++ "px")
                        , style "transform" "scale(var(--asp-scale, 1))"
                        , style "transform-origin" "0 0"
                        , class "block border-0"
                        ]
                        []
                    ]

            Nothing ->
                div [ class "h-64 rounded-lg bg-kalk/30" ] []
        ]



-- DIALOGS


cropDialog : Model -> Html Msg
cropDialog model =
    case ( model.crop, model.crop |> Maybe.andThen (\c -> Photos.get c.photoId model.photos) ) of
        ( Just c, Just photo ) ->
            let
                size =
                    { width = photo.width, height = photo.height }

                current =
                    Crop.find photo.quality c.kind

                -- Polite, and only its words change, so a screen reader
                -- hears it as the frame crosses a limit, not on every move.
                sharpnessLine =
                    case Dict.get c.kind c.rects of
                        Just r ->
                            let
                                floorLine =
                                    if Crop.atFloor current r then
                                        [ Text.atFloor ]

                                    else
                                        []

                                ( kind, lines ) =
                                    case Crop.sharpness current (Dict.get c.kind photo.crops) r of
                                        Crop.Sharp ->
                                            ( "sharp", Text.sharp :: floorLine )

                                        Crop.Soft ->
                                            ( "soft", Text.softCrop :: floorLine ++ [ Text.askForOriginal ] )

                                        Crop.TooSmall ->
                                            ( "error", [ Text.cropTooSmall ] )
                            in
                            Ui.verdict kind [] lines

                        Nothing ->
                            text ""

                cropErrors =
                    case c.error of
                        Just (Api.Problems ps) ->
                            List.map Text.problem ps

                        Just e ->
                            [ Text.error e ]

                        Nothing ->
                            []

                navButton lbl aria msg =
                    Html.button
                        [ A.type_ "button"
                        , onClick msg
                        , attribute "aria-label" aria
                        , class "min-w-12 min-h-12 rounded-lg border-2 border-sort bg-white font-bold text-xl hover:bg-brown/20 focus-visible:outline-3 focus-visible:outline-offset-2 focus-visible:outline-sort"
                        ]
                        [ text lbl ]
            in
            Ui.modal { id = "beskjaer-dialog", title = "Beskjær bildet", onClose = CloseCrop, wide = True }
                [ Ui.help [] "Flytt og dra i rammen, eller bruk knappene. Det som er innenfor rammen, vises på nettsiden. Trykk på et av de små bildene for å velge hvilket utsnitt du endrer."
                , div [ class "flex flex-wrap gap-6 items-start" ]
                    [ div [ class "flex-[2_1_22rem] min-w-0 flex flex-col gap-4" ]
                        [ p [ class "font-bold text-lg" ] [ text (current.name ++ ": " ++ current.usedFor) ]
                        , Keyed.node "div"
                            [ class "bg-sort rounded-lg overflow-hidden" ]
                            [ ( String.fromInt c.photoId ++ "/" ++ c.kind
                              , node "asp-cropper"
                                    [ attribute "src" photo.previewUrl
                                    , attribute "source-width" (String.fromInt photo.width)
                                    , attribute "source-height" (String.fromInt photo.height)
                                    , attribute "aspect" (Crop.aspectString current)
                                    , attribute "rect" (Dict.get c.kind c.rects |> Maybe.map Crop.rectString |> Maybe.withDefault "")
                                    , on "crop" (D.field "detail" rectDecoder |> D.map (GotCrop c.kind))
                                    ]
                                    []
                              )
                            ]
                        , div [ class "flex flex-wrap gap-2 items-center", attribute "role" "group", attribute "aria-label" "Flytt eller endre rammen" ]
                            [ navButton "←" "Flytt rammen til venstre" (CropNudge -1 0)
                            , navButton "→" "Flytt rammen til høyre" (CropNudge 1 0)
                            , navButton "↑" "Flytt rammen opp" (CropNudge 0 -1)
                            , navButton "↓" "Flytt rammen ned" (CropNudge 0 1)
                            , Ui.secondaryButton [] (CropResize 1.1) "Større"
                            , Ui.secondaryButton [] (CropResize (1 / 1.1)) "Mindre"
                            ]
                        ]
                    , div [ class "flex-[1_1_14rem] flex flex-col gap-4", attribute "role" "group", attribute "aria-label" "Utsnitt" ]
                        (List.map (cropChoice photo c) (Crop.kinds photo.quality))
                    ]
                , Ui.errorText "beskjaer-feil" cropErrors
                , Ui.help [] "Beskjæringen gjelder med en gang du trykker «Bruk», også hvis arrangementet allerede er publisert."
                , div [ class "sticky bottom-0 -mx-4 md:-mx-8 -mb-4 md:-mb-8 px-4 md:px-8 py-4 bg-white flex flex-wrap gap-3 justify-end items-center border-t border-brown" ]
                    -- In the sticky footer, beside Bruk: in sight on any
                    -- screen, and its changing length never moves the frame
                    -- she is dragging.
                    [ div [ A.id "beskjaer-skarphet", attribute "aria-live" "polite", attribute "role" "status", class "flex-[1_1_18rem] min-w-0" ] [ sharpnessLine ]
                    , Ui.secondaryButton [] CloseCrop "Avbryt"
                    , Ui.primaryButton [ A.disabled c.saving, A.autofocus True ]
                        SaveCrop
                        (if c.saving then
                            "Lagrer …"

                         else
                            "Bruk"
                        )
                    ]
                ]

        _ ->
            text ""


cropChoice : Photo -> CropDialog -> Crop.Kind -> Html Msg
cropChoice photo c kind =
    let
        boxWidth =
            if kind.key == "card" then
                112

            else
                208

        r =
            Dict.get kind.key c.rects |> Maybe.withDefault { x = 0, y = 0, w = photo.width, h = photo.height }

        t =
            Crop.thumb { width = photo.width, height = photo.height } boxWidth r

        px v =
            String.fromFloat v ++ "px"

        picture =
            div [ class "relative overflow-hidden rounded bg-kalk/40 shrink-0", style "width" (px boxWidth), style "height" (px t.boxHeight) ]
                [ img
                    [ A.src photo.previewUrl
                    , A.alt ""
                    , class "absolute max-w-none"
                    , style "width" (px t.imgWidth)
                    , style "height" (px t.imgHeight)
                    , style "left" (px t.left)
                    , style "top" (px t.top)
                    ]
                    []
                ]
    in
    if Crop.usable kind { width = photo.width, height = photo.height } then
        Html.button
            [ A.type_ "button"
            , onClick (CropKind kind.key)
            , attribute "aria-pressed"
                (if c.kind == kind.key then
                    "true"

                 else
                    "false"
                )
            , class "flex flex-col gap-2 items-start text-left p-2 rounded-lg border-2 border-transparent aria-pressed:border-sort aria-pressed:bg-brown/20 hover:bg-brown/10 focus-visible:outline-3 focus-visible:outline-offset-2 focus-visible:outline-sort"
            ]
            [ picture
            , span [ class "font-bold" ]
                [ text
                    (kind.usedFor
                        ++ (if c.kind == kind.key then
                                " (endres nå)"

                            else
                                ""
                           )
                    )
                ]
            ]

    else
        div [ class "flex flex-col gap-2 items-start p-2", attribute "data-crop-unusable" kind.key ]
            [ span [ class "font-bold" ] [ text kind.usedFor ]
            , if kind.quality.soft then
                Ui.verdict "soft" [] [ fallbackText kind ++ " " ++ Text.softWhole, Text.askForOriginal ]

              else
                Ui.help [] (fallbackText kind)
            ]


rectDecoder : D.Decoder Rect
rectDecoder =
    D.map4 Rect (D.field "x" D.int) (D.field "y" D.int) (D.field "w" D.int) (D.field "h" D.int)


conflictDialog : Model -> Html Msg
conflictDialog model =
    case model.conflict of
        Just _ ->
            Ui.modal { id = "konflikt-dialog", title = "Noen andre har endret arrangementet", onClose = Ignored, wide = False }
                [ p [ class "text-lg" ] [ text "Mens du skrev, har noen andre lagret endringer i dette arrangementet. Det du har skrevet er ikke lagret ennå, men det er ikke borte." ]
                , p [ class "text-lg" ] [ text "Vil du lagre din versjon over deres, eller hente deres versjon og la din være?" ]
                , div [ class "flex flex-col sm:flex-row gap-3 justify-end" ]
                    [ Ui.secondaryButton [] TakeTheirs "Hent deres versjon"
                    , Ui.primaryButton [ A.autofocus True ] KeepMine "Lagre min versjon"
                    ]
                ]

        Nothing ->
            text ""
