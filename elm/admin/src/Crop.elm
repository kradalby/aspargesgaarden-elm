module Crop exposing
    ( Kind
    , Size
    , Thumb
    , aspectString
    , kinds
    , minWidth
    , nudge
    , rectString
    , resize
    , thumb
    , tooSmallForShare
    )

{-| The three crops every main photo carries, and the rectangle arithmetic
behind the crop dialog's buttons and small previews. Rectangles are in
upright source pixels, as the server stores them.
-}

import Api exposing (Rect)


type alias Kind =
    { key : String

    -- Short tab name, then where the crop shows up, in her words.
    , name : String
    , usedFor : String
    , ratioW : Int
    , ratioH : Int
    }


kinds : List Kind
kinds =
    [ Kind "card" "Listen" "I listen over arrangementer" 4 5
    , Kind "banner" "Toppen" "Øverst på siden til arrangementet" 16 9
    , Kind "share" "Deling" "Når noen deler lenken, for eksempel på Facebook" 1200 630
    ]


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


{-| The smallest width the server accepts for a crop (its smallest output).
-}
minWidth : Int
minWidth =
    480


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
at least minWidth wide (or the whole photo, if smaller) and inside the photo.
-}
resize : Kind -> Size -> Float -> Rect -> Rect
resize kind photo factor r =
    let
        a =
            aspect kind

        wanted =
            round (toFloat r.w * factor)
                |> max (min minWidth photo.width)
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


{-| Facebook shows the share image at 1200 px wide; a smaller crop still
works but looks soft there.
-}
tooSmallForShare : Rect -> Bool
tooSmallForShare r =
    r.w < 1200
