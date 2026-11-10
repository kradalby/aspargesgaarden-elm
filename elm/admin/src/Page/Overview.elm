module Page.Overview exposing (Model, Msg, Out(..), init, update, view)

{-| "Arrangementer": what is coming, what is not published yet, and what has
been, with the actions she needs on each.
-}

import Api exposing (Event, EventLists)
import Html exposing (Html, a, article, div, h3, img, p, section, text)
import Html.Attributes as A exposing (class)
import Route
import Text
import Ui


type alias Model =
    { lists : Status
    , showAllPast : Bool
    , confirmDelete : Maybe Event
    , working : Bool
    , failure : Maybe String
    , announce : String
    }


type Status
    = Loading
    | Failed Api.Error
    | Loaded EventLists


type Msg
    = Got (Result Api.Error EventLists)
    | Reload
    | ShowAllPast
    | Copy Int
    | Copied (Result Api.Error Event)
    | AskDelete Event
    | CancelDelete
    | ConfirmDelete
    | Deleted Int (Result Api.Error ())
    | Focused


{-| Main navigates; a copy opens in the editor.
-}
type Out
    = Open Int


init : ( Model, Cmd Msg )
init =
    ( { lists = Loading, showAllPast = False, confirmDelete = Nothing, working = False, failure = Nothing, announce = "" }
    , Api.getEvents Got
    )


update : Msg -> Model -> ( Model, Cmd Msg, Maybe Out )
update msg model =
    case msg of
        Got (Ok lists) ->
            ( { model | lists = Loaded lists }, Cmd.none, Nothing )

        Got (Err e) ->
            ( { model | lists = Failed e }, Cmd.none, Nothing )

        Reload ->
            ( { model | lists = Loading }, Api.getEvents Got, Nothing )

        ShowAllPast ->
            ( { model | showAllPast = True }, Cmd.none, Nothing )

        Copy id ->
            ( { model | working = True, failure = Nothing, announce = "Lager kopi …" }, Api.copyEvent id Copied, Nothing )

        Copied (Ok e) ->
            ( { model | working = False }, Cmd.none, Just (Open e.id) )

        Copied (Err e) ->
            ( { model | working = False, failure = Just ("Kopien ble ikke laget. " ++ Text.error e) }, Cmd.none, Nothing )

        AskDelete e ->
            ( { model | confirmDelete = Just e, failure = Nothing }, Cmd.none, Nothing )

        CancelDelete ->
            ( { model | confirmDelete = Nothing }
            , model.confirmDelete |> Maybe.map (\e -> focus ("slett-" ++ String.fromInt e.id)) |> Maybe.withDefault Cmd.none
            , Nothing
            )

        ConfirmDelete ->
            case model.confirmDelete of
                Just e ->
                    ( { model | working = True }, Api.deleteEvent e.id (Deleted e.id), Nothing )

                Nothing ->
                    ( model, Cmd.none, Nothing )

        Deleted id (Ok ()) ->
            let
                drop =
                    List.filter (\e -> e.id /= id)
            in
            ( { model
                | working = False
                , confirmDelete = Nothing
                , announce = "Arrangementet er slettet."
                , lists =
                    case model.lists of
                        Loaded l ->
                            Loaded { upcoming = drop l.upcoming, drafts = drop l.drafts, past = drop l.past }

                        other ->
                            other
              }
            , focus "nytt-arrangement"
            , Nothing
            )

        Deleted _ (Err e) ->
            ( { model | working = False, confirmDelete = Nothing, failure = Just ("Arrangementet ble ikke slettet. " ++ Text.error e) }, Cmd.none, Nothing )

        Focused ->
            ( model, Cmd.none, Nothing )


focus : String -> Cmd Msg
focus =
    Ui.focus Focused



-- VIEW


view : Model -> Html Msg
view model =
    div [ class "flex flex-col gap-10" ]
        [ div [ class "flex flex-wrap gap-4 items-center justify-between" ]
            [ Ui.h1 "Arrangementer"
            , a [ Route.href Route.NewEvent, A.id "nytt-arrangement", class "inline-flex items-center gap-2 min-h-14 px-7 rounded-lg bg-sort text-white text-lg font-bold no-underline hover:bg-mork-eukalyptus focus-visible:outline-3 focus-visible:outline-offset-2 focus-visible:outline-sort" ]
                [ Html.span [ A.attribute "aria-hidden" "true", class "text-2xl leading-none" ] [ text "+" ], text "Nytt arrangement" ]
            ]
        , Ui.liveRegion model.announce
        , case model.failure of
            Just f ->
                Ui.notice "error" [ p [ A.attribute "role" "alert" ] [ text f ] ]

            Nothing ->
                text ""
        , case model.lists of
            Loading ->
                Ui.spinnerText "Henter arrangementene …"

            Failed e ->
                Ui.notice "error"
                    [ p [ A.attribute "role" "alert" ] [ text ("Klarte ikke å hente arrangementene. " ++ Text.error e) ]
                    , div [] [ Ui.primaryButton [] Reload "Prøv igjen" ]
                    ]

            Loaded lists ->
                div [ class "flex flex-col gap-12" ]
                    [ listSection model
                        { title = "Kommende"
                        , help = "Disse vises på nettsiden nå."
                        , empty = "Ingen kommende arrangementer er publisert."
                        , events = lists.upcoming
                        }
                    , listSection model
                        { title = "Kladder"
                        , help = "Kladder vises ikke på nettsiden før du trykker «Publiser»."
                        , empty = "Ingen kladder."
                        , events = lists.drafts
                        }
                    , listSection model
                        { title = "Tidligere"
                        , help = "Skal du ha det samme arrangementet igjen? Trykk «Lag kopi» og bytt dato."
                        , empty = "Ingen tidligere arrangementer."
                        , events =
                            if model.showAllPast then
                                lists.past

                            else
                                List.take 5 lists.past
                        }
                    , if not model.showAllPast && List.length lists.past > 5 then
                        div [] [ Ui.secondaryButton [] ShowAllPast ("Vis alle " ++ String.fromInt (List.length lists.past) ++ " tidligere arrangementer") ]

                      else
                        text ""
                    ]
        , case model.confirmDelete of
            Just e ->
                Ui.modal { id = "slett-dialog", title = "Slette arrangementet?", onClose = CancelDelete, wide = False }
                    [ p [ class "text-lg" ] [ text ("«" ++ titleOf e ++ "» blir borte for godt. Det kan ikke angres.") ]
                    , if e.status == "published" then
                        p [ class "text-lg font-bold" ] [ text "Siden forsvinner også fra nettsiden, og lenker til den slutter å virke." ]

                      else
                        text ""
                    , Ui.help [] "Bildene blir liggende, så de kan brukes igjen."
                    , div [ class "flex flex-wrap gap-3 justify-end" ]
                        [ Ui.secondaryButton [ A.autofocus True ] CancelDelete "Nei, behold det"
                        , Ui.dangerButton [ A.disabled model.working ]
                            ConfirmDelete
                            (if model.working then
                                "Sletter …"

                             else
                                "Ja, slett"
                            )
                        ]
                    ]

            Nothing ->
                text ""
        ]


titleOf : Event -> String
titleOf e =
    if String.trim e.title == "" then
        "Uten tittel"

    else
        e.title


listSection : Model -> { title : String, help : String, empty : String, events : List Event } -> Html Msg
listSection model s =
    section [ class "flex flex-col gap-4", A.attribute "aria-labelledby" ("liste-" ++ s.title) ]
        [ div []
            [ Ui.h2 [ A.id ("liste-" ++ s.title) ] s.title
            , Ui.help [] s.help
            ]
        , if List.isEmpty s.events then
            p [ class "text-brown-text italic" ] [ text s.empty ]

          else
            div [ class "flex flex-col gap-3" ] (List.map (eventRow model) s.events)
        ]


eventRow : Model -> Event -> Html Msg
eventRow model e =
    article
        [ class "bg-white border border-brown rounded-xl p-4 flex flex-wrap gap-4 md:gap-6 items-center"
        , A.attribute "data-event" (String.fromInt e.id)
        ]
        [ case e.mainPhoto of
            Just ph ->
                if ph.status.previewReady then
                    img [ A.src ph.thumbUrl, A.alt "", class "w-20 h-24 md:w-28 md:h-36 object-cover rounded-lg shrink-0" ] []

                else
                    div [ class "w-20 h-24 md:w-28 md:h-36 rounded-lg shrink-0 bg-kalk/40" ] []

            Nothing ->
                div [ class "w-20 h-24 md:w-28 md:h-36 rounded-lg shrink-0 bg-kalk/40 flex items-center justify-center text-sm text-center p-1" ] [ text "Uten bilde" ]
        , div [ class "flex-1 min-w-48 flex flex-col gap-1.5" ]
            [ div [ class "flex flex-wrap gap-2" ]
                [ if e.status == "published" then
                    Ui.badge "published" "Publisert"

                  else
                    Ui.badge "draft" "Kladd – vises ikke"
                , if e.showInMenu then
                    Ui.badge "" "I menyen"

                  else
                    text ""
                ]
            , h3 [ class "font-display text-tre text-2xl leading-tight" ] [ text (titleOf e) ]
            , p [ class "text-brown-text" ]
                [ text
                    (if e.dateText == "" then
                        "Dato er ikke satt"

                     else
                        e.dateText
                    )
                ]
            ]
        , div [ class "flex flex-wrap gap-2 w-full md:w-auto" ]
            [ a
                [ Route.href (Route.EditEvent e.id)
                , class "inline-flex items-center justify-center min-h-12 px-5 rounded-lg border-2 border-sort bg-sort text-white font-bold no-underline hover:bg-mork-eukalyptus focus-visible:outline-3 focus-visible:outline-offset-2 focus-visible:outline-sort"
                , A.attribute "aria-label" ("Endre " ++ titleOf e)
                ]
                [ text "Endre" ]
            , Ui.secondaryButton [ A.disabled model.working, A.attribute "aria-label" ("Lag kopi av " ++ titleOf e) ] (Copy e.id) "Lag kopi"
            , if e.status == "published" && e.url /= "" then
                Ui.linkButton [ A.href e.url, A.target "_blank", A.rel "noopener", A.attribute "aria-label" ("Se " ++ titleOf e ++ " på nettsiden (åpnes i ny fane)") ] "Se på nettsiden"

              else
                text ""
            , Ui.dangerButton [ A.id ("slett-" ++ String.fromInt e.id), A.attribute "aria-label" ("Slett " ++ titleOf e) ] (AskDelete e) "Slett"
            ]
        ]
