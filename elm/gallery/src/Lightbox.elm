module Lightbox exposing
    ( Action(..)
    , Credit
    , Flags
    , Key
    , Photo
    , Size
    , Source
    , apply
    , fit
    , flagsDecoder
    , keyAction
    , swipeAction
    , swipeThreshold
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

The frame is the old gallery's: the whole window on a phone, four fifths of
it from 768 px (of the window less 16 px from 1280), at most 1024 px, with
p-4 pb-10 (Gallery.figure). On a phone the arrows sit below the photo, from
768 px beside it. A window too low for photo and caption, a phone on its
side, gives the photo its height, and the caption is scrolled to.

-}
fit : Size -> Size -> Size
fit viewport photo =
    let
        frame =
            if viewport.width >= 1280 then
                min (widest + 32) ((viewport.width - 16) * 4 // 5)

            else if viewport.width >= 768 then
                viewport.width * 4 // 5

            else
                viewport.width

        -- the padding above the photo, the credit and a caption line under
        -- it, and the padding under them
        chrome =
            16 + 24 + 24 + 40

        room =
            { width = max 1 (frame - 32)
            , height =
                max 1
                    (if viewport.height < 500 then
                        viewport.height - 32

                     else if viewport.width >= 768 then
                        viewport.height - 32 - chrome

                     else
                        -- Close above, the arrows below.
                        viewport.height - 64 - 88 - chrome
                    )
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
