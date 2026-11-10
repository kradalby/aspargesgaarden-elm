module CropTest exposing (suite)

import Crop
import Expect
import Fuzz
import Test exposing (Test, describe, fuzz2, test)


photo : Crop.Size
photo =
    { width = 4000, height = 3000 }


banner : Crop.Kind
banner =
    { key = "banner", name = "", usedFor = "", ratioW = 16, ratioH = 9 }


inside : Crop.Size -> { x : Int, y : Int, w : Int, h : Int } -> Bool
inside p r =
    r.x >= 0 && r.y >= 0 && r.x + r.w <= p.width && r.y + r.h <= p.height


suite : Test
suite =
    describe "Crop"
        [ test "nudge moves by a twentieth of the frame" <|
            \_ ->
                Crop.nudge photo 1 0 { x = 0, y = 0, w = 2000, h = 1125 }
                    |> .x
                    |> Expect.equal 100
        , test "nudge stops at the edge" <|
            \_ ->
                Crop.nudge photo 1 1 { x = 1950, y = 1850, w = 2000, h = 1125 }
                    |> (\r -> ( r.x, r.y ))
                    |> Expect.equal ( 2000, 1875 )
        , test "resize keeps the centre and the shape" <|
            \_ ->
                Crop.resize banner photo 0.5 { x = 0, y = 375, w = 4000, h = 2250 }
                    |> Expect.equal { x = 1000, y = 938, w = 2000, h = 1125 }
        , test "resize never goes below the smallest output" <|
            \_ ->
                Crop.resize banner photo 0.01 { x = 0, y = 375, w = 4000, h = 2250 }
                    |> .w
                    |> Expect.equal Crop.minWidth
        , fuzz2 (Fuzz.floatRange 0.2 3) (Fuzz.intRange 0 2000) "resize always stays inside the photo with the right shape" <|
            \factor x ->
                let
                    r =
                        Crop.resize banner photo factor { x = x, y = 0, w = 1600, h = 900 }
                in
                Expect.all
                    [ \_ -> Expect.equal True (inside photo r)
                    , \_ -> Expect.atMost 1 (abs (toFloat r.w / (16 / 9) - toFloat r.h))
                    ]
                    ()
        , test "thumb shows exactly the crop in the box" <|
            \_ ->
                Crop.thumb photo 200 { x = 1000, y = 500, w = 2000, h = 1000 }
                    |> Expect.equal { boxHeight = 100, imgWidth = 400, imgHeight = 300, left = -100, top = -50 }
        , test "share warning below Facebook's width" <|
            \_ ->
                ( Crop.tooSmallForShare { x = 0, y = 0, w = 1199, h = 630 }, Crop.tooSmallForShare { x = 0, y = 0, w = 1200, h = 630 } )
                    |> Expect.equal ( True, False )
        ]
