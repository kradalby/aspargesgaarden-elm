module Lightbox exposing
    ( Action(..)
    , Credit
    , Ends
    , Flags
    , Key
    , Photo
    , Size
    , Source
    , Tab
    , apply
    , fit
    , flagsDecoder
    , keyAction
    , swipeAction
    , swipeThreshold
    , wrapFocus
    )

{-| The lightbox's rules, free of the DOM so they can be tested: what the
server hands over, which photo comes next, what a key or a swipe means, and
how large a photo is shown.
-}

import Array exposing (Array)
import Json.Decode as Decode exposing (Decoder)


{-| One encoding of a photo, as a `<source>` of a `<picture>`.
-}
type alias Source =
    { type_ : String
    , srcset : String
    }


type alias Credit =
    { name : String
    , link : Maybe String
    }


{-| A photo as ui.Gallery describes it in `data-gallery`.
-}
type alias Photo =
    { alt : String
    , size : Size
    , src : String
    , srcset : String
    , sources : List Source
    , credit : Maybe Credit
    }


type alias Size =
    { width : Int
    , height : Int
    }


type alias Flags =
    { id : String
    , photos : Array Photo
    , viewport : Size
    }


flagsDecoder : Decoder Flags
flagsDecoder =
    Decode.map3 Flags
        (Decode.field "id" Decode.string)
        (Decode.field "photos" (Decode.array photoDecoder))
        (Decode.field "viewport" sizeDecoder)


photoDecoder : Decoder Photo
photoDecoder =
    Decode.map6 Photo
        (Decode.field "alt" Decode.string)
        sizeDecoder
        (Decode.field "src" Decode.string)
        (Decode.field "srcset" Decode.string)
        (Decode.field "sources" (Decode.list sourceDecoder))
        (Decode.maybe (Decode.field "by" Decode.string)
            |> Decode.andThen
                (\by ->
                    case by of
                        Nothing ->
                            Decode.succeed Nothing

                        Just name ->
                            Decode.maybe (Decode.field "link" Decode.string)
                                |> Decode.map (Just << Credit name)
                )
        )


sourceDecoder : Decoder Source
sourceDecoder =
    Decode.map2 Source
        (Decode.field "type" Decode.string)
        (Decode.field "srcset" Decode.string)


sizeDecoder : Decoder Size
sizeDecoder =
    Decode.map2 Size
        (Decode.field "width" Decode.int)
        (Decode.field "height" Decode.int)


type Action
    = Previous
    | Next
    | Close


{-| The photo shown after an action, or Nothing when the lightbox closes. The
ends wrap around, as the old gallery's did.
-}
apply : Int -> Action -> Int -> Maybe Int
apply count action index =
    if count <= 0 then
        Nothing

    else
        case action of
            Previous ->
                Just (modBy count (index - 1))

            Next ->
                Just (modBy count (index + 1))

            Close ->
                Nothing


type alias Key =
    { key : String
    , modified : Bool
    }


{-| Arrows browse and Escape closes. With a modifier held the key belongs to
the browser (Alt+Left is "back").
-}
keyAction : Key -> Maybe Action
keyAction { key, modified } =
    if modified then
        Nothing

    else
        case key of
            "ArrowLeft" ->
                Just Previous

            "ArrowRight" ->
                Just Next

            "Escape" ->
                Just Close

            _ ->
                Nothing


{-| How far a finger must travel sideways before it counts as a swipe; less
is a tap or a wobble.
-}
swipeThreshold : Float
swipeThreshold =
    50


{-| A finger that moved mostly sideways, far enough, browses: towards the
left shows the next photo, as on a phone's photo app.
-}
swipeAction : { dx : Float, dy : Float } -> Maybe Action
swipeAction { dx, dy } =
    if abs dx < swipeThreshold || abs dx <= abs dy then
        Nothing

    else if dx < 0 then
        Just Next

    else
        Just Previous


{-| The size a photo is shown at in a viewport: as large as fits beside the
controls and above the caption, never larger than the photo itself. The
browser then picks the variant from the srcset for this width.

The margins mirror the classes in Gallery.view: on a phone the arrows sit
below the photo, from 768 px beside it, and the frame stops at 1024 px.

-}
fit : Size -> Size -> Size
fit viewport photo =
    let
        wide =
            viewport.width >= 768

        -- frame padding (p-2 / p-4) on both sides
        frame =
            if wide then
                32

            else
                16

        -- two lines of caption and the gap above it
        caption =
            48

        room =
            if wide then
                { width = max 1 (min widest (viewport.width - 2 * 80 - frame))
                , height = max 1 (viewport.height - 2 * 32 - frame - caption)
                }

            else
                { width = max 1 (viewport.width - frame)
                , height = max 1 (viewport.height - 64 - 88 - frame - caption)
                }

        w =
            max 1 photo.width

        h =
            max 1 photo.height
    in
    if w <= room.width && h <= room.height then
        { width = w, height = h }

    else if room.width * h <= room.height * w then
        { width = room.width, height = max 1 (h * room.width // w) }

    else
        { width = max 1 (w * room.height // h), height = room.height }


{-| The widest a photo is ever shown: the 1024 px frame less its padding.
images.Full's SharpWidth is this number; a test in ui reads it from here.
-}
widest : Int
widest =
    992


{-| The ids of the first and last focusable control of the lightbox.
-}
type alias Ends =
    { first : String
    , last : String
    }


{-| A keydown: its key, Shift, and the id of the focused element.
-}
type alias Tab =
    { key : String
    , shift : Bool
    , target : String
    }


{-| Where focus goes instead when Tab would leave the lightbox: round to the
other end. Nothing lets the browser move it as usual.
-}
wrapFocus : Ends -> Tab -> Maybe String
wrapFocus ends { key, shift, target } =
    if key /= "Tab" then
        Nothing

    else if shift && target == ends.first then
        Just ends.last

    else if not shift && target == ends.last then
        Just ends.first

    else
        Nothing
