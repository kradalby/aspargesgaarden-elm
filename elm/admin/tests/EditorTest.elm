module EditorTest exposing (suite)

import Api
import Dict
import Expect
import Fixture
import Form
import Page.Editor as Editor exposing (Action(..), Msg(..), Request(..))
import Photos
import Test exposing (Test, describe, test)


step : Msg -> Editor.Model -> Editor.Model
step msg model =
    Editor.update msg model |> (\( m, _, _ ) -> m)


loaded : Editor.Model
loaded =
    Editor.init (Just 4) |> Tuple.first |> step (Loaded (Ok (Fixture.event { id = 4, updatedAt = "1", summary = "Hennes" })))


withMain : Int -> Int -> Editor.Model
withMain width height =
    let
        spec =
            Fixture.spec 2
    in
    Editor.init (Just 4)
        |> Tuple.first
        |> step (Loaded (Ok (Fixture.eventWith (Just { spec | width = width, height = height }) { id = 4, updatedAt = "1", summary = "" })))


published : Api.Event -> Api.Event
published e =
    { e | status = "published" }


mainEvent : Api.Event
mainEvent =
    Fixture.eventWith (Just (Fixture.spec 2)) { id = 4, updatedAt = "1", summary = "" }


changedPhoto : Api.Photo
changedPhoto =
    let
        p =
            Fixture.photo 2
    in
    { p | alt = "Deres tekst", updatedAt = "3" }


theirs : Api.Event
theirs =
    Fixture.event { id = 4, updatedAt = "2", summary = "Deres" }


suite : Test
suite =
    describe "Editor"
        [ describe "two editors"
            [ test "keeping hers saves it over theirs before publishing" <|
                \_ ->
                    loaded
                        |> step (Run Publish)
                        |> step (Done Publish (Err (Api.Conflict theirs [])))
                        |> step KeepMine
                        |> Editor.nextRequest Publish
                        |> Expect.equal (Update 4)
            , test "keeping hers sends it with their version as precondition" <|
                \_ ->
                    loaded
                        |> step (Run Publish)
                        |> step (Done Publish (Err (Api.Conflict theirs [])))
                        |> step KeepMine
                        |> (\m -> ( m.event |> Maybe.map .updatedAt, m.form.summary, m.busy ))
                        |> Expect.equal ( Just "2", "Hennes", Just Publish )
            , test "closing the question keeps her text, unsaved" <|
                \_ ->
                    loaded
                        |> step (Set Editor.Summary "Ny tekst")
                        |> step (Run SaveDraft)
                        |> step (Saved SaveDraft (Err (Api.Conflict theirs [])))
                        |> step CloseConflict
                        |> (\m -> ( m.conflict == Nothing, m.form.summary, Editor.isDirty m ))
                        |> Expect.equal ( True, "Ny tekst", True )
            , test "taking theirs loads it" <|
                \_ ->
                    loaded
                        |> step (Run Publish)
                        |> step (Done Publish (Err (Api.Conflict theirs [])))
                        |> step TakeTheirs
                        |> (\m -> ( m.form.summary, Editor.isDirty m ))
                        |> Expect.equal ( "Deres", False )
            ]
        , describe "a photo changed elsewhere"
            [ test "using theirs takes their words for that photo and saves the rest" <|
                \_ ->
                    withMain 1300 900
                        |> step (SetAlt 2 "Min tekst")
                        |> step (Run SaveDraft)
                        |> step (Saved SaveDraft (Err (Api.Conflict mainEvent [ changedPhoto ])))
                        |> step TakeTheirs
                        |> (\m -> ( m.form.mainPhoto |> Maybe.map (\r -> ( r.alt, r.updatedAt )), m.busy ))
                        |> Expect.equal ( Just ( "Deres tekst", "3" ), Just SaveDraft )
            , test "using hers keeps her words at their version" <|
                \_ ->
                    withMain 1300 900
                        |> step (SetAlt 2 "Min tekst")
                        |> step (Run SaveDraft)
                        |> step (Saved SaveDraft (Err (Api.Conflict mainEvent [ changedPhoto ])))
                        |> step KeepMine
                        |> (\m -> ( m.form.mainPhoto |> Maybe.map (\r -> ( r.alt, r.updatedAt )), Editor.nextRequest SaveDraft m ))
                        |> Expect.equal ( Just ( "Min tekst", "3" ), Update 4 )
            ]
        , test "the preview is drawn again once its photo is ready" <|
            \_ ->
                let
                    spec =
                        Fixture.spec 2

                    before =
                        Editor.init (Just 4)
                            |> Tuple.first
                            |> step (Loaded (Ok (Fixture.eventWith (Just { spec | state = "preparing" }) { id = 4, updatedAt = "1", summary = "" })))

                    after =
                        step (PhotosMsg (Photos.GotStatuses (Ok (Dict.singleton 2 { state = Api.Ready, done = 9, total = 9, previewReady = True })))) before
                in
                Expect.greaterThan before.preview.seq after.preview.seq
        , describe "what a save sends first"
            [ test "an unchanged event is published at once" <|
                \_ -> loaded |> Editor.nextRequest Publish |> Expect.equal (PublishOnly 4)
            , test "a changed one is saved first" <|
                \_ -> loaded |> step (Set Editor.Title "Nytt") |> Editor.nextRequest Publish |> Expect.equal (Update 4)
            , test "a new one is created" <|
                \_ -> Editor.init Nothing |> Tuple.first |> Editor.nextRequest SaveDraft |> Expect.equal Create
            ]
        , describe "a hidden tab"
            [ test "saves a draft with changes" <|
                \_ -> loaded |> step (Set Editor.Summary "Ny") |> step Hidden |> .busy |> Expect.equal (Just SaveDraft)
            , test "leaves a draft without changes alone" <|
                \_ -> loaded |> step Hidden |> .busy |> Expect.equal Nothing
            , test "never saves a published event by itself" <|
                \_ ->
                    loaded
                        |> step (Done Publish (Ok (published (Fixture.event { id = 4, updatedAt = "2", summary = "" }))))
                        |> step (Set Editor.Summary "Ny")
                        |> step Hidden
                        |> .busy
                        |> Expect.equal Nothing
            ]
        , describe "the crop dialog"
            [ test "opens on the list crop when the photo is large enough" <|
                \_ -> withMain 1300 900 |> step OpenCrop |> .crop |> Maybe.map .kind |> Expect.equal (Just "card")
            , test "opens on the first crop the photo is large enough for" <|
                -- 500 px high: a 4:5 crop would be 400 wide, under 480.
                \_ -> withMain 1300 500 |> step OpenCrop |> .crop |> Maybe.map .kind |> Expect.equal (Just "banner")
            , test "does not open for a photo too small for every crop" <|
                \_ -> withMain 600 300 |> step OpenCrop |> .crop |> Expect.equal Nothing
            ]
        , test "the form is what was loaded" <|
            \_ -> loaded.form |> Expect.equal (Form.fromEvent (Fixture.event { id = 4, updatedAt = "1", summary = "Hennes" }))
        ]
