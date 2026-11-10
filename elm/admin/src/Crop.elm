module Crop exposing
    ( Kind
    , Sharpness(..)
    , Size
    , Thumb
    , aspectString
    , atFloor
    , find
    , kinds
    , nudge
    , rectString
    , resize
    , sharpness
    , thumb
    , usable
    )

{-| The three crops every main photo carries, and the rectangle arithmetic
behind the crop dialog's buttons and small previews. Rectangles are in
upright source pixels, as the server stores them.
-}

import Api exposing (Quality, Rect)


type alias Kind =
    { key : String

    -- Short tab name, then where the crop shows up, in her words.
    , name : String
    , usedFor : String
    , ratioW : Int
    , ratioH : Int

    -- The server's widths for the kind, and how this photo fares.
    , quality : Quality
    }


{-| The kinds of a photo, with the widths its server answer carries.
-}
kinds : Api.Qualities -> List Kind
kinds q =
    [ Kind "card" "Listen" "I listen over arrangementer" 4 5 q.card
    , Kind "banner" "Toppen" "Øverst på siden til arrangementet" 16 9 q.banner
    , Kind "share" "Deling" "Når noen deler lenken, for eksempel på Facebook" 1200 630 q.share
    ]


{-| The kind with the key; the card for an unknown one.
-}
find : Api.Qualities -> String -> Kind
find q key =
    case List.filter (\k -> k.key == key) (kinds q) of
        k :: _ ->
            k

        [] ->
            Kind "card" "Listen" "I listen over arrangementer" 4 5 q.card


{-| How a frame will look on the site. TooSmall is what the server refuses.
-}
type Sharpness
    = Sharp
    | Soft
    | TooSmall


{-| stored is the crop the photo has now. Sent back unchanged it is always
accepted, so even under today's minimum it is only soft.
-}
sharpness : Kind -> Maybe Rect -> Rect -> Sharpness
sharpness kind stored r =
    if r.w < kind.quality.minWidth && stored /= Just r then
        TooSmall

    else if r.w < kind.quality.sharpWidth then
        Soft

    else
        Sharp


{-| Whether "Mindre" can shrink the frame no further.
-}
atFloor : Kind -> Rect -> Bool
atFloor kind r =
    r.w <= kind.quality.minWidth


type alias Size =
    { width : Int, height : Int }


aspect : Kind -> Float
aspect k =
    toFloat k.ratioW / toFloat k.ratioH


aspectString : Kind -> String
aspectString k =
    String.fromInt k.ratioW ++ ":" ++ String.fromInt k.ratioH


rectString : Rect -> String
rectString r =
    String.join "," (List.map String.fromInt [ r.x, r.y, r.w, r.h ])


{-| Whether the photo is large enough for the kind at all. The server keeps
no crop of a kind a photo is too small for (the page then uses another
picture), so the dialog offers none.
-}
usable : Kind -> Size -> Bool
usable kind photo =
    let
        centredWidth =
            if photo.width * kind.ratioH > photo.height * kind.ratioW then
                photo.height * kind.ratioW // kind.ratioH

            else
                photo.width
    in
    centredWidth >= kind.quality.minWidth


{-| Moves the frame by dx, dy steps of a twentieth of its own size, kept
inside the photo.
-}
nudge : Size -> Int -> Int -> Rect -> Rect
nudge photo dx dy r =
    let
        step n =
            max 1 (n // 20)
    in
    { r
        | x = clamp 0 (photo.width - r.w) (r.x + dx * step r.w)
        , y = clamp 0 (photo.height - r.h) (r.y + dy * step r.h)
    }


{-| Scales the frame around its centre by factor, keeping the kind's shape,
at least the kind's minimum width (or the whole photo, if smaller) and
inside the photo. A frame already under the minimum (an old crop) is never
grown by shrinking it.
-}
resize : Kind -> Size -> Float -> Rect -> Rect
resize kind photo factor r =
    let
        a =
            aspect kind

        wanted =
            round (toFloat r.w * factor)
                |> max (min (min kind.quality.minWidth photo.width) r.w)
                |> min photo.width

        ( w, h ) =
            if round (toFloat wanted / a) > photo.height then
                ( round (toFloat photo.height * a), photo.height )

            else
                ( wanted, round (toFloat wanted / a) )

        cx =
            toFloat r.x + toFloat r.w / 2

        cy =
            toFloat r.y + toFloat r.h / 2
    in
    { x = clamp 0 (photo.width - w) (round (cx - toFloat w / 2))
    , y = clamp 0 (photo.height - h) (round (cy - toFloat h / 2))
    , w = w
    , h = h
    }


{-| How to draw the crop r of a photo in a box boxWidth pixels wide, by
showing the whole (preview) image scaled and shifted inside a clipping box.
-}
type alias Thumb =
    { boxHeight : Float, imgWidth : Float, imgHeight : Float, left : Float, top : Float }


thumb : Size -> Float -> Rect -> Thumb
thumb photo boxWidth r =
    let
        s =
            boxWidth / toFloat (max 1 r.w)
    in
    { boxHeight = toFloat r.h * s
    , imgWidth = toFloat photo.width * s
    , imgHeight = toFloat photo.height * s
    , left = -(toFloat r.x * s)
    , top = -(toFloat r.y * s)
    }
