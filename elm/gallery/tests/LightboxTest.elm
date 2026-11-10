module LightboxTest exposing (suite)

import Array
import Expect
import Json.Decode as Decode
import Lightbox exposing (Action(..))
import Test exposing (Test, describe, test)


suite : Test
suite =
    describe "Lightbox"
        [ describe "apply"
            [ test "next" <| \_ -> Lightbox.apply 3 Next 0 |> Expect.equal (Just 1)
            , test "previous" <| \_ -> Lightbox.apply 3 Previous 2 |> Expect.equal (Just 1)
            , test "next wraps at the end" <| \_ -> Lightbox.apply 3 Next 2 |> Expect.equal (Just 0)
            , test "previous wraps at the start" <| \_ -> Lightbox.apply 3 Previous 0 |> Expect.equal (Just 2)
            , test "one photo stays" <| \_ -> Lightbox.apply 1 Next 0 |> Expect.equal (Just 0)
            , test "close" <| \_ -> Lightbox.apply 3 Close 1 |> Expect.equal Nothing
            , test "empty gallery never opens" <| \_ -> Lightbox.apply 0 Next 0 |> Expect.equal Nothing
            ]
        , describe "keyAction"
            [ test "left" <| \_ -> Lightbox.keyAction { key = "ArrowLeft", modified = False } |> Expect.equal (Just Previous)
            , test "right" <| \_ -> Lightbox.keyAction { key = "ArrowRight", modified = False } |> Expect.equal (Just Next)
            , test "escape" <| \_ -> Lightbox.keyAction { key = "Escape", modified = False } |> Expect.equal (Just Close)
            , test "other keys" <| \_ -> Lightbox.keyAction { key = "Enter", modified = False } |> Expect.equal Nothing
            , test "alt+left is the browser's back" <| \_ -> Lightbox.keyAction { key = "ArrowLeft", modified = True } |> Expect.equal Nothing
            ]
        , describe "swipeAction"
            [ test "finger to the left shows the next photo" <| \_ -> Lightbox.swipeAction { dx = -80, dy = 10 } |> Expect.equal (Just Next)
            , test "finger to the right shows the previous photo" <| \_ -> Lightbox.swipeAction { dx = 80, dy = -10 } |> Expect.equal (Just Previous)
            , test "a short move is a tap" <| \_ -> Lightbox.swipeAction { dx = -49, dy = 0 } |> Expect.equal Nothing
            , test "the threshold itself counts" <| \_ -> Lightbox.swipeAction { dx = -50, dy = 0 } |> Expect.equal (Just Next)
            , test "a mostly vertical move is not a swipe" <| \_ -> Lightbox.swipeAction { dx = 60, dy = 90 } |> Expect.equal Nothing
            ]
        , describe "wrapFocus"
            [ test "tab on the last control goes to the first" <|
                \_ -> Lightbox.wrapFocus ends { key = "Tab", shift = False, target = "g-next" } |> Expect.equal (Just "g-close")
            , test "shift+tab on the first goes to the last" <|
                \_ -> Lightbox.wrapFocus ends { key = "Tab", shift = True, target = "g-close" } |> Expect.equal (Just "g-next")
            , test "tab inside moves as usual" <|
                \_ -> Lightbox.wrapFocus ends { key = "Tab", shift = False, target = "g-close" } |> Expect.equal Nothing
            , test "shift+tab inside moves as usual" <|
                \_ -> Lightbox.wrapFocus ends { key = "Tab", shift = True, target = "g-next" } |> Expect.equal Nothing
            , test "other keys are not ours" <|
                \_ -> Lightbox.wrapFocus ends { key = "Enter", shift = False, target = "g-next" } |> Expect.equal Nothing
            , test "tab on the dialog itself (the photo was clicked) goes to the first" <|
                \_ -> Lightbox.wrapFocus ends { key = "Tab", shift = False, target = "g" } |> Expect.equal (Just "g-close")
            , test "shift+tab on the dialog itself goes to the last" <|
                \_ -> Lightbox.wrapFocus ends { key = "Tab", shift = True, target = "g" } |> Expect.equal (Just "g-next")
            , test "one photo, no credit link: tab stays on Close" <|
                \_ -> Lightbox.wrapFocus { ends | last = "g-close" } { key = "Tab", shift = False, target = "g-close" } |> Expect.equal (Just "g-close")
            ]
        , describe "fit"
            [ test "landscape on a phone fills the width" <|
                \_ -> Lightbox.fit { width = 390, height = 844 } { width = 2000, height = 1333 } |> Expect.equal { width = 374, height = 249 }
            , test "portrait on a tall phone still fills the width" <|
                \_ -> Lightbox.fit { width = 390, height = 844 } { width = 1333, height = 2000 } |> Expect.equal { width = 374, height = 561 }
            , test "portrait on a short phone is bound by the height" <|
                \_ -> Lightbox.fit { width = 390, height = 600 } { width = 1333, height = 2000 } |> Expect.equal { width = 255, height = 384 }
            , test "landscape on a desktop stops at the frame" <|
                \_ -> Lightbox.fit { width = 1920, height = 1080 } { width = 2000, height = 1333 } |> Expect.equal { width = 992, height = 661 }
            , test "portrait on a laptop is bound by the height" <|
                \_ -> Lightbox.fit { width = 1280, height = 800 } { width = 1333, height = 2000 } |> Expect.equal { width = 437, height = 656 }
            , test "a small photo is not enlarged" <|
                \_ -> Lightbox.fit { width = 1920, height = 1080 } { width = 400, height = 300 } |> Expect.equal { width = 400, height = 300 }
            , test "a tiny viewport still gives a box" <|
                \_ -> Lightbox.fit { width = 10, height = 10 } { width = 2000, height = 1333 } |> Expect.equal { width = 1, height = 1 }
            ]
        , describe "flagsDecoder"
            [ test "reads what ui.Gallery writes" <|
                \_ ->
                    Decode.decodeString Lightbox.flagsDecoder flags
                        |> Result.map (\f -> ( f.id, Array.toList f.photos, f.viewport ))
                        |> Expect.equal
                            (Ok
                                ( "gallery-0"
                                , [ { alt = "Låven pyntet til bryllup"
                                    , size = { width = 2000, height = 1333 }
                                    , src = "/img/a-1200.jpeg"
                                    , srcset = "/img/a-480.jpeg 480w, /img/a-1200.jpeg 1200w"
                                    , sources = [ { type_ = "image/webp", srcset = "/img/a-480.webp 480w" } ]
                                    , credit = Just { name = "Lucky Thirteen", link = Just "https://luckythirteen.no/" }
                                    }
                                  , { alt = "Fjøset"
                                    , size = { width = 800, height = 600 }
                                    , src = "/img/b-800.jpeg"
                                    , srcset = "/img/b-800.jpeg 800w"
                                    , sources = []
                                    , credit = Just { name = "Sophia", link = Nothing }
                                    }
                                  , { alt = "Hagen"
                                    , size = { width = 800, height = 600 }
                                    , src = "/img/c-800.jpeg"
                                    , srcset = "/img/c-800.jpeg 800w"
                                    , sources = []
                                    , credit = Nothing
                                    }
                                  ]
                                , { width = 390, height = 844 }
                                )
                            )
            ]
        ]


ends : Lightbox.Ends
ends =
    { first = "g-close", last = "g-next", dialog = "g" }


{-| boot.js's flags around the JSON ui.Gallery writes (gallery\_test.go
renders the same shape).
-}
flags : String
flags =
    """
    { "id": "gallery-0"
    , "viewport": { "width": 390, "height": 844 }
    , "photos":
        [ { "alt": "Låven pyntet til bryllup", "width": 2000, "height": 1333
          , "src": "/img/a-1200.jpeg", "srcset": "/img/a-480.jpeg 480w, /img/a-1200.jpeg 1200w"
          , "sources": [ { "type": "image/webp", "srcset": "/img/a-480.webp 480w" } ]
          , "by": "Lucky Thirteen", "link": "https://luckythirteen.no/" }
        , { "alt": "Fjøset", "width": 800, "height": 600
          , "src": "/img/b-800.jpeg", "srcset": "/img/b-800.jpeg 800w", "sources": []
          , "by": "Sophia" }
        , { "alt": "Hagen", "width": 800, "height": 600
          , "src": "/img/c-800.jpeg", "srcset": "/img/c-800.jpeg 800w", "sources": [] }
        ]
    }
    """
