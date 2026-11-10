port module Admin exposing (main)

{-| The admin app for aspargesgaarden.no, served under /admin/. Admin owns
the address, the header and the "leave without saving?" question; each
screen lives in Page.\*.
-}

import Api
import Browser
import Browser.Dom as Dom
import Browser.Navigation as Nav
import Html exposing (Html, a, div, header, main_, nav, p, span, text)
import Html.Attributes as A exposing (attribute, class)
import Page.Editor as Editor
import Page.Gallery as Gallery
import Page.Overview as Overview
import Route exposing (Route)
import Task
import Text
import Ui
import Url exposing (Url)
import Venue


{-| Tells the page whether there is unsaved work, so the browser asks before
the tab is closed or reloaded.
-}
port setDirty : Bool -> Cmd msg


type alias Model =
    { key : Nav.Key
    , url : Url
    , me : Maybe Api.Me
    , page : Page
    , leave : Maybe Url
    , leaving : Bool
    , dirty : Bool

    -- Why the server will not let this person in, in words; the screens
    -- give way to it, since every request would fail the same way.
    , denied : Maybe String
    }


type Page
    = NotFound
    | Overview Overview.Model
    | Editor Editor.Model
    | Gallery Gallery.Model


type Msg
    = LinkClicked Browser.UrlRequest
    | UrlChanged Url
    | GotMe (Result Api.Error Api.Me)
    | StayHere
    | LeaveAnyway
    | OverviewMsg Overview.Msg
    | EditorMsg Editor.Msg
    | GalleryMsg Gallery.Msg


main : Program () Model Msg
main =
    Browser.application
        { init = init
        , view = view
        , update = \msg model -> update msg model |> reportDirty
        , subscriptions = subscriptions
        , onUrlRequest = LinkClicked
        , onUrlChange = UrlChanged
        }


init : () -> Url -> Nav.Key -> ( Model, Cmd Msg )
init _ url key =
    let
        ( page, cmd ) =
            pageFor (Route.fromUrl url)
    in
    ( { key = key, url = url, me = Nothing, page = page, leave = Nothing, leaving = False, dirty = False, denied = Nothing }
    , Cmd.batch [ cmd, Api.getMe GotMe ]
    )


pageFor : Maybe Route -> ( Page, Cmd Msg )
pageFor route =
    case route of
        Just Route.Overview ->
            Overview.init |> wrap Overview OverviewMsg

        Just Route.NewEvent ->
            Editor.init Nothing |> wrap Editor EditorMsg

        Just (Route.EditEvent id) ->
            Editor.init (Just id) |> wrap Editor EditorMsg

        Just Route.Gallery ->
            Gallery.init |> wrap Gallery GalleryMsg

        Nothing ->
            ( NotFound, Cmd.none )


wrap : (m -> Page) -> (msg -> Msg) -> ( m, Cmd msg ) -> ( Page, Cmd Msg )
wrap toPage toMsg ( m, cmd ) =
    ( toPage m, Cmd.map toMsg cmd )


isDirty : Page -> Bool
isDirty page =
    case page of
        Editor m ->
            Editor.isDirty m

        Gallery m ->
            Gallery.isDirty m

        _ ->
            False


reportDirty : ( Model, Cmd Msg ) -> ( Model, Cmd Msg )
reportDirty ( model, cmd ) =
    let
        dirty =
            isDirty model.page
    in
    if dirty == model.dirty then
        ( model, cmd )

    else
        ( { model | dirty = dirty }, Cmd.batch [ cmd, setDirty dirty ] )


update : Msg -> Model -> ( Model, Cmd Msg )
update msg model =
    case ( msg, model.page ) of
        ( LinkClicked (Browser.Internal url), _ ) ->
            if url.path == model.url.path then
                -- Only the skip link points inside the page.
                ( model
                , url.fragment
                    |> Maybe.map (\id -> Task.attempt (\_ -> StayHere) (Dom.focus id))
                    |> Maybe.withDefault Cmd.none
                )

            else if isDirty model.page then
                ( { model | leave = Just url }, Cmd.none )

            else
                ( model, Nav.pushUrl model.key (Url.toString url) )

        ( LinkClicked (Browser.External href), _ ) ->
            ( model, Nav.load href )

        ( UrlChanged url, page ) ->
            let
                route =
                    Route.fromUrl url
            in
            case ( route, page ) of
                ( Just (Route.EditEvent id), Editor m ) ->
                    if Editor.eventId m == Just id then
                        -- The address of a new event after its first save.
                        ( { model | url = url }, Cmd.none )

                    else
                        go url route model

                _ ->
                    if url.path == model.url.path || model.leave == Just url then
                        -- Nav.forward reports the address it leaves at once,
                        -- before the browser has moved; that is not a new request.
                        ( model, Cmd.none )

                    else if isDirty page && not model.leaving then
                        -- The browser's back button: the address has already
                        -- changed, so step forward again and ask first.
                        ( { model | leave = Just url }, Nav.forward model.key 1 )

                    else
                        go url route model

        ( GotMe (Ok me), _ ) ->
            ( { model | me = Just me }, Cmd.none )

        ( GotMe (Err (Api.Problems ps)), _ ) ->
            case List.filter (\p -> p.code == "forbidden" || p.code == "wrong_host") ps of
                p :: _ ->
                    ( { model | denied = Just (Text.problem p) }, Cmd.none )

                [] ->
                    ( model, Cmd.none )

        ( GotMe (Err _), _ ) ->
            ( model, Cmd.none )

        ( StayHere, _ ) ->
            ( { model | leave = Nothing }, Cmd.none )

        ( LeaveAnyway, _ ) ->
            case model.leave of
                Just url ->
                    ( { model | leave = Nothing, leaving = True }, Nav.pushUrl model.key (Url.toString url) )

                Nothing ->
                    ( model, Cmd.none )

        ( OverviewMsg sub, Overview m ) ->
            let
                ( m2, cmd, out ) =
                    Overview.update sub m
            in
            ( { model | page = Overview m2 }
            , Cmd.batch
                [ Cmd.map OverviewMsg cmd
                , case out of
                    Just (Overview.Open id) ->
                        Nav.pushUrl model.key (Route.toString (Route.EditEvent id))

                    Nothing ->
                        Cmd.none
                ]
            )

        ( EditorMsg sub, Editor m ) ->
            let
                ( m2, cmd, out ) =
                    Editor.update sub m
            in
            ( { model | page = Editor m2 }
            , Cmd.batch
                [ Cmd.map EditorMsg cmd
                , case out of
                    Just (Editor.Created id) ->
                        Nav.replaceUrl model.key (Route.toString (Route.EditEvent id))

                    Nothing ->
                        Cmd.none
                ]
            )

        ( GalleryMsg sub, Gallery m ) ->
            Gallery.update sub m |> wrap Gallery GalleryMsg |> (\( p, c ) -> ( { model | page = p }, c ))

        _ ->
            ( model, Cmd.none )


go : Url -> Maybe Route -> Model -> ( Model, Cmd Msg )
go url route model =
    let
        ( page, cmd ) =
            pageFor route
    in
    ( { model | url = url, page = page, leaving = False }, cmd )


subscriptions : Model -> Sub Msg
subscriptions model =
    case model.page of
        Editor m ->
            Sub.map EditorMsg (Editor.subscriptions m)

        Gallery m ->
            Sub.map GalleryMsg (Gallery.subscriptions m)

        _ ->
            Sub.none



-- VIEW


view : Model -> Browser.Document Msg
view model =
    { title = title model.page ++ " – " ++ Venue.name ++ " admin"
    , body =
        [ div [ class "min-h-screen bg-brown/20 text-sort font-sans text-lg leading-normal" ]
            [ a [ A.href "#innhold", class "sr-only focus:not-sr-only focus:absolute focus:top-2 focus:left-2 focus:z-50 focus:bg-white focus:p-3 focus:rounded-lg" ] [ text "Hopp til innholdet" ]
            , siteHeader model
            , main_ [ A.id "innhold", A.tabindex -1, class "max-w-7xl mx-auto px-4 md:px-8 py-6 md:py-10 focus:outline-none" ]
                [ case ( model.denied, model.page ) of
                    ( Just why, _ ) ->
                        div [ class "flex flex-col gap-4 max-w-2xl" ]
                            [ Ui.h1 "Ingen tilgang"
                            , p [ A.attribute "role" "alert", class "text-xl" ] [ text why ]
                            , p [] [ text "Du kan fortsatt se nettsiden som vanlig." ]
                            ]

                    ( Nothing, NotFound ) ->
                        div [ class "flex flex-col gap-4" ]
                            [ Ui.h1 "Fant ikke siden"
                            , a [ Route.href Route.Overview, class "underline font-bold" ] [ text "Til arrangementene" ]
                            ]

                    ( Nothing, Overview m ) ->
                        Html.map OverviewMsg (Overview.view m)

                    ( Nothing, Editor m ) ->
                        Html.map EditorMsg (Editor.view m)

                    ( Nothing, Gallery m ) ->
                        Html.map GalleryMsg (Gallery.view m)
                ]
            , leaveDialog model
            ]
        ]
    }


title : Page -> String
title page =
    case page of
        NotFound ->
            "Fant ikke siden"

        Overview _ ->
            "Arrangementer"

        Editor _ ->
            "Endre arrangement"

        Gallery _ ->
            "Galleri"


siteHeader : Model -> Html Msg
siteHeader model =
    let
        navLink route label active =
            a
                [ Route.href route
                , class "inline-flex items-center min-h-11 px-3 rounded-lg font-bold uppercase tracking-wide text-base no-underline hover:bg-brown/20 focus-visible:outline-3 focus-visible:outline-sort"
                , class
                    (if active then
                        "underline underline-offset-8 decoration-2"

                     else
                        ""
                    )
                , attribute "aria-current"
                    (if active then
                        "page"

                     else
                        "false"
                    )
                ]
                [ text label ]

        onEvents =
            case model.page of
                Overview _ ->
                    True

                Editor _ ->
                    True

                _ ->
                    False

        onGallery =
            case model.page of
                Gallery _ ->
                    True

                _ ->
                    False
    in
    header [ class "bg-white border-b border-brown" ]
        [ div [ class "max-w-7xl mx-auto px-4 md:px-8 py-3 flex flex-wrap gap-x-8 gap-y-2 items-center justify-between" ]
            [ div [ class "flex flex-wrap items-baseline gap-x-3" ]
                [ span [ class "font-display text-3xl text-sort" ] [ text Venue.name ]
                , span [ class "text-brown-text text-base" ] [ text "Administrasjon" ]
                ]
            , nav [ attribute "aria-label" "Hovedmeny", class "flex flex-wrap gap-2" ]
                [ navLink Route.Overview "Arrangementer" onEvents
                , navLink Route.Gallery "Galleri" onGallery
                ]
            , div [ class "flex flex-wrap gap-x-5 gap-y-1 items-center text-base" ]
                [ case model.me of
                    Just me ->
                        p [ class "text-brown-text" ] [ text ("Logget inn som " ++ me.name) ]

                    Nothing ->
                        text ""
                , case model.me of
                    Just me ->
                        a [ A.href me.siteUrl, A.target "_blank", A.rel "noopener", class "inline-flex items-center min-h-11 font-bold underline underline-offset-4" ] [ text "Se nettsiden (ny fane)" ]

                    Nothing ->
                        text ""
                ]
            ]
        ]


leaveDialog : Model -> Html Msg
leaveDialog model =
    case model.leave of
        Just _ ->
            Ui.modal { id = "forlat-dialog", title = "Du har endringer som ikke er lagret", onClose = StayHere, wide = False }
                [ p [ class "text-lg" ] [ text "Går du videre nå, blir endringene borte." ]
                , div [ class "flex flex-col sm:flex-row gap-3 justify-end" ]
                    [ Ui.dangerButton [] LeaveAnyway "Gå videre uten å lagre"
                    , Ui.primaryButton [ A.autofocus True ] StayHere "Bli her og lagre"
                    ]
                ]

        Nothing ->
            text ""
