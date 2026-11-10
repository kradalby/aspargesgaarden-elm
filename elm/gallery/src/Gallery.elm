port module Gallery exposing (main)

{-| The lightbox over one server-rendered gallery (ui.Gallery). boot.js
starts one instance per `[data-gallery]` and opens it when a photo link is
clicked; while closed it renders nothing, so mounting moves no pixel.
-}

import Array
import Browser
import Browser.Dom
import Browser.Events
import Html exposing (Html)
import Html.Attributes as Attr
import Html.Events as Events
import Json.Decode as Decode exposing (Decoder)
import Lightbox exposing (Action(..), Photo, Size)
import Task


{-| The index of the photo link the visitor clicked.
-}
port open : (Int -> msg) -> Sub msg


{-| The lightbox closed; boot.js gives focus back to the clicked link.
-}
port closed : () -> Cmd msg


main : Program Decode.Value Model Msg
main =
    Browser.element
        { init = init
        , update = update
        , view = view
        , subscriptions = subscriptions
        }


type alias Model =
    { id : String
    , photos : Array.Array Photo
    , viewport : Size
    , current : Maybe Int
    , drag : Maybe Drag
    }


{-| A finger on the screen: where it went down and how far it has moved.
-}
type alias Drag =
    { pointer : Int
    , x0 : Float
    , y0 : Float
    , dx : Float
    , dy : Float
    }


type alias Pointer =
    { id : Int
    , x : Float
    , y : Float
    }


type Msg
    = Opened Int
    | Did Action
    | Resized Int Int
    | PointerDown Pointer
    | PointerMove Pointer
    | PointerUp Pointer
    | PointerCancel
    | FocusEdge String
    | NoOp


init : Decode.Value -> ( Model, Cmd Msg )
init value =
    case Decode.decodeValue Lightbox.flagsDecoder value of
        Ok flags ->
            ( { id = flags.id
              , photos = flags.photos
              , viewport = flags.viewport
              , current = Nothing
              , drag = Nothing
              }
            , Cmd.none
            )

        -- The links still lead to the large photos; nothing to enhance.
        Err _ ->
            ( { id = "", photos = Array.empty, viewport = Size 0 0, current = Nothing, drag = Nothing }
            , Cmd.none
            )


update : Msg -> Model -> ( Model, Cmd Msg )
update msg model =
    case msg of
        Opened index ->
            if index >= 0 && index < Array.length model.photos then
                ( { model | current = Just index, drag = Nothing }, focus (closeId model) )

            else
                ( model, Cmd.none )

        Did action ->
            case model.current of
                Nothing ->
                    ( model, Cmd.none )

                Just index ->
                    case Lightbox.apply (Array.length model.photos) action index of
                        Nothing ->
                            ( { model | current = Nothing, drag = Nothing }, closed () )

                        next ->
                            ( { model | current = next, drag = Nothing }, Cmd.none )

        Resized width height ->
            ( { model | viewport = Size width height }, Cmd.none )

        PointerDown p ->
            ( { model | drag = Just { pointer = p.id, x0 = p.x, y0 = p.y, dx = 0, dy = 0 } }, Cmd.none )

        PointerMove p ->
            ( { model | drag = Maybe.map (moved p) model.drag }, Cmd.none )

        PointerUp p ->
            case Maybe.map (moved p) model.drag of
                Just d ->
                    case Lightbox.swipeAction { dx = d.dx, dy = d.dy } of
                        Just action ->
                            update (Did action) { model | drag = Nothing }

                        Nothing ->
                            ( { model | drag = Nothing }, Cmd.none )

                Nothing ->
                    ( model, Cmd.none )

        PointerCancel ->
            ( { model | drag = Nothing }, Cmd.none )

        FocusEdge id ->
            ( model, focus id )

        NoOp ->
            ( model, Cmd.none )


moved : Pointer -> Drag -> Drag
moved p d =
    if p.id == d.pointer then
        { d | dx = p.x - d.x0, dy = p.y - d.y0 }

    else
        d


focus : String -> Cmd Msg
focus id =
    Task.attempt (\_ -> NoOp) (Browser.Dom.focus id)


closeId : Model -> String
closeId model =
    model.id ++ "-close"


nextId : Model -> String
nextId model =
    model.id ++ "-next"


{-| The window size is followed while closed too: a phone turned between
two photos must not open the next one fitted to the old window.
-}
subscriptions : Model -> Sub Msg
subscriptions model =
    case model.current of
        Nothing ->
            Sub.batch [ open Opened, Browser.Events.onResize Resized ]

        Just _ ->
            Sub.batch
                [ Browser.Events.onKeyDown (Decode.andThen keyMsg keyDecoder)
                , Browser.Events.onResize Resized
                ]


keyDecoder : Decoder Lightbox.Key
keyDecoder =
    Decode.map2 Lightbox.Key
        (Decode.field "key" Decode.string)
        (Decode.map3 (\a c m -> a || c || m)
            (Decode.field "altKey" Decode.bool)
            (Decode.field "ctrlKey" Decode.bool)
            (Decode.field "metaKey" Decode.bool)
        )


keyMsg : Lightbox.Key -> Decoder Msg
keyMsg key =
    case Lightbox.keyAction key of
        Just action ->
            Decode.succeed (Did action)

        Nothing ->
            Decode.fail "not ours"



-- VIEW


view : Model -> Html Msg
view model =
    case model.current |> Maybe.andThen (\i -> Maybe.map (Tuple.pair i) (Array.get i model.photos)) of
        Nothing ->
            Html.text ""

        Just ( index, photo ) ->
            let
                count =
                    Array.length model.photos

                several =
                    count > 1

                neighbours =
                    if several then
                        List.filterMap (\a -> Lightbox.apply count a index |> Maybe.andThen (\i -> Array.get i model.photos))
                            [ Previous, Next ]

                    else
                        []
            in
            Html.div
                [ Attr.id model.id
                , Attr.attribute "role" "dialog"
                , Attr.attribute "aria-modal" "true"
                , Attr.attribute "aria-label" (position index count)

                -- A click on the photo focuses the dialog rather than the
                -- page under it, so Tab stays inside (trapTab).
                , Attr.tabindex -1
                , Attr.attribute "data-lightbox" ""

                -- Only a click on the backdrop itself closes; the photo and
                -- the buttons are other targets.
                , Attr.attribute "data-backdrop" ""
                , Events.on "click"
                    (Decode.at [ "target", "dataset", "backdrop" ] Decode.string
                        |> Decode.map (\_ -> Did Close)
                    )
                , Events.preventDefaultOn "keydown" (trapTab (focusEnds model photo))
                , Events.on "pointerdown" (touchPointer PointerDown)
                , Events.on "pointermove" (touchPointer PointerMove)
                , Events.on "pointerup" (touchPointer PointerUp)
                , Events.on "pointercancel" (Decode.succeed PointerCancel)
                , Attr.class "fixed inset-0 z-50 flex flex-col items-center justify-center bg-brown/60 touch-pinch-zoom motion-safe:transition-opacity motion-safe:duration-200 starting:opacity-0"
                ]
                ([ -- First, so the same node stays and its new text is read
                   -- out: focus stays on the arrow while the photo changes.
                   Html.p [ Attr.class "sr-only", Attr.attribute "aria-live" "polite" ]
                    [ Html.text (position index count ++ ": " ++ photo.alt) ]
                 , button (closeId model) "Lukk" "✕" (Did Close) "top-2 right-2"
                 , figure model photo
                 ]
                    ++ (if several then
                            [ button (model.id ++ "-previous") "Forrige bilde" "❮" (Did Previous) "left-2 bottom-6 pr-1 md:top-1/2 md:-translate-y-1/2"
                            , button (nextId model) "Neste bilde" "❯" (Did Next) "right-2 bottom-6 pl-1 md:top-1/2 md:-translate-y-1/2"
                            ]

                        else
                            []
                       )
                    ++ [ Html.div [ Attr.class "hidden" ] (List.map (picture model.viewport) neighbours) ]
                )


position : Int -> Int -> String
position index count =
    "Bilde " ++ String.fromInt (index + 1) ++ " av " ++ String.fromInt count


{-| The first and last control in tab order: Close, then the credit link,
then the arrows.
-}
focusEnds : Model -> Photo -> Lightbox.Ends
focusEnds model photo =
    { dialog = model.id
    , first = closeId model
    , last =
        if Array.length model.photos > 1 then
            nextId model

        else if Maybe.andThen .link photo.credit /= Nothing then
            creditId model

        else
            closeId model
    }


creditId : Model -> String
creditId model =
    model.id ++ "-credit"


{-| Tab past either end goes round to the other, so focus cannot wander to
the page underneath. Deciding on keydown, before the browser moves focus,
holds however fast the keys come.
-}
trapTab : Lightbox.Ends -> Decoder ( Msg, Bool )
trapTab ends =
    Decode.map3 Lightbox.Tab
        (Decode.field "key" Decode.string)
        (Decode.field "shiftKey" Decode.bool)
        (Decode.at [ "target", "id" ] Decode.string)
        |> Decode.andThen
            (\tab ->
                case Lightbox.wrapFocus ends tab of
                    Just id ->
                        Decode.succeed ( FocusEdge id, True )

                    Nothing ->
                        Decode.fail "default"
            )


button : String -> String -> String -> Msg -> String -> Html Msg
button id label glyph msg place =
    Html.button
        [ Attr.id id
        , Attr.type_ "button"
        , Attr.attribute "aria-label" label
        , Events.onClick msg
        , Attr.class ("absolute w-14 h-14 rounded-full bg-white text-4xl text-sort flex flex-col justify-center text-center " ++ place)
        ]
        [ Html.span [ Attr.attribute "aria-hidden" "true" ] [ Html.text glyph ] ]


figure : Model -> Photo -> Html Msg
figure model photo =
    let
        ( shift, settle ) =
            case model.drag of
                Just d ->
                    if abs d.dx > abs d.dy then
                        ( d.dx, "" )

                    else
                        ( 0, "" )

                Nothing ->
                    ( 0, " motion-safe:transition-transform motion-safe:duration-200" )
    in
    Html.node "figure"
        [ Attr.class ("bg-white p-2 md:p-4" ++ settle)

        -- CSSOM, not a style attribute: allowed under style-src 'self'.
        , Attr.style "transform" ("translateX(" ++ String.fromFloat shift ++ "px)")
        ]
        [ picture model.viewport photo
        , Html.node "figcaption"
            [ Attr.class "w-0 min-w-full mt-2 flex justify-between gap-4 text-sm leading-5 text-sort" ]
            -- The <img> carries the same text as its alt.
            [ Html.span [ Attr.class "line-clamp-2", Attr.attribute "aria-hidden" "true" ] [ Html.text photo.alt ]
            , credit (creditId model) photo
            ]
        ]


credit : String -> Photo -> Html msg
credit id photo =
    case photo.credit of
        Nothing ->
            Html.text ""

        Just { name, link } ->
            let
                label =
                    "Fotograf: " ++ name
            in
            case link of
                Nothing ->
                    Html.span [ Attr.class "shrink-0" ] [ Html.text label ]

                Just href ->
                    Html.a [ Attr.id id, Attr.href href, Attr.target "_blank", Attr.rel "noopener noreferrer", Attr.class "shrink-0 underline" ] [ Html.text label ]


{-| The photo at the size that fits; `sizes` tells the browser that width so
it fetches the matching variant, and width/height hold the box before the
bytes arrive.
-}
picture : Size -> Photo -> Html msg
picture viewport photo =
    let
        shown =
            Lightbox.fit viewport photo.size

        sizes =
            String.fromInt shown.width ++ "px"
    in
    Html.node "picture"
        []
        (List.map
            (\s -> Html.node "source" [ Attr.type_ s.type_, Attr.attribute "srcset" s.srcset, Attr.attribute "sizes" sizes ] [])
            photo.sources
            ++ [ Html.img
                    [ Attr.src photo.src
                    , Attr.attribute "srcset" photo.srcset
                    , Attr.attribute "sizes" sizes
                    , Attr.width shown.width
                    , Attr.height shown.height
                    , Attr.alt photo.alt
                    , Attr.attribute "draggable" "false"
                    , Attr.class "block select-none"
                    ]
                    []
               ]
        )


{-| A touch or pen pointer; the mouse clicks the buttons instead of dragging.
-}
touchPointer : (Pointer -> Msg) -> Decoder Msg
touchPointer toMsg =
    Decode.field "pointerType" Decode.string
        |> Decode.andThen
            (\kind ->
                if kind == "mouse" then
                    Decode.fail "mouse"

                else
                    Decode.map3 Pointer
                        (Decode.field "pointerId" Decode.int)
                        (Decode.field "clientX" Decode.float)
                        (Decode.field "clientY" Decode.float)
                        |> Decode.map toMsg
            )
