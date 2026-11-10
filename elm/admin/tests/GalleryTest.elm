module GalleryTest exposing (suite)

import Api
import Dict
import Expect
import Fixture
import Page.Gallery as Gallery exposing (Msg(..), OrderSave(..))
import Photos
import Test exposing (Test, describe, test)


loaded : List Int -> Gallery.Model
loaded ids =
    Gallery.init |> Tuple.first |> step (Loaded (Ok (List.map Fixture.photo ids)))


step : Msg -> Gallery.Model -> Gallery.Model
step msg model =
    Gallery.update msg model |> Tuple.first


suite : Test
suite =
    describe "Gallery"
        [ describe "merge"
            [ test "a photo someone else added is kept, at the end" <|
                \_ -> Gallery.merge { mine = [ 2, 1 ], known = [ 1, 2 ], server = [ 1, 2, 9 ] } |> Expect.equal [ 2, 1, 9 ]
            , test "her own new photo is kept" <|
                \_ -> Gallery.merge { mine = [ 1, 2, 7 ], known = [ 1, 2 ], server = [ 1, 2 ] } |> Expect.equal [ 1, 2, 7 ]
            , test "one she took out stays out" <|
                \_ -> Gallery.merge { mine = [ 2 ], known = [ 1, 2 ], server = [ 1, 2 ] } |> Expect.equal [ 2 ]
            , test "one taken out elsewhere stays out" <|
                \_ -> Gallery.merge { mine = [ 2, 1 ], known = [ 1, 2 ], server = [ 2 ] } |> Expect.equal [ 2 ]
            ]
        , describe "order saves"
            [ test "a second move waits for the save that is out" <|
                \_ ->
                    loaded [ 1, 2, 3 ]
                        |> step (Move 0 1)
                        |> step (Move 1 1)
                        |> (\m -> ( m.orderSave, m.order ))
                        |> Expect.equal ( Reading, [ 2, 3, 1 ] )
            , test "an answer for an older order sends the newer one again" <|
                \_ ->
                    loaded [ 1, 2, 3 ]
                        |> step (Move 0 1)
                        |> step (GotCurrent (Ok (List.map Fixture.photo [ 1, 2, 3 ])))
                        |> step (Move 1 1)
                        |> step (OrderSaved (Ok (List.map Fixture.photo [ 2, 1, 3 ])))
                        |> (\m -> ( m.orderSave, m.savedOrder, m.order ))
                        |> Expect.equal ( Reading, [ 2, 1, 3 ], [ 2, 3, 1 ] )
            , test "a change made while the gallery is read is in the save" <|
                \_ ->
                    loaded [ 1, 2, 3 ]
                        |> step (Move 0 1)
                        |> step (RemoveFromGallery 3)
                        |> step (GotCurrent (Ok (List.map Fixture.photo [ 1, 2, 3 ])))
                        |> (\m -> ( m.orderSave, m.order ))
                        |> Expect.equal ( Writing, [ 2, 1 ] )
            , test "nothing is left to save once the server has her order" <|
                \_ ->
                    loaded [ 1, 2 ]
                        |> step (Move 0 1)
                        |> step (GotCurrent (Ok (List.map Fixture.photo [ 1, 2 ])))
                        |> step (OrderSaved (Ok (List.map Fixture.photo [ 2, 1 ])))
                        |> (\m -> ( m.orderSave, Gallery.isDirty m ))
                        |> Expect.equal ( Idle, False )
            , test "another admin's new photo survives her move" <|
                \_ ->
                    loaded [ 1, 2 ]
                        |> step (Move 0 1)
                        |> step (GotCurrent (Ok (List.map Fixture.photo [ 1, 2, 9 ])))
                        |> (\m -> ( m.order, Photos.get 9 m.photos |> Maybe.map .id ))
                        |> Expect.equal ( [ 2, 1, 9 ], Just 9 )
            ]
        , describe "a photo's description and photographer"
            [ test "a photographer change that failed is still hers, and unsaved" <|
                \_ ->
                    loaded [ 1 ]
                        |> step (SetPhotographer 1 (Just 5))
                        |> step (PhotoSaved 1 (Err Api.Network))
                        |> (\m -> ( Gallery.creditOf 1 m, Gallery.isDirty m ))
                        |> Expect.equal ( Just 5, True )
            , test "a saved photographer is no longer pending" <|
                \_ ->
                    loaded [ 1 ]
                        |> step (SetPhotographer 1 (Just 5))
                        |> step (PhotoSaved 1 (Ok (Fixture.photoWith { id = 1, width = 1300, height = 900, alt = "Bilde 1", photographer = Just 5, state = "ready" })))
                        |> (\m -> ( Gallery.creditOf 1 m, Dict.isEmpty m.credits, Gallery.isDirty m ))
                        |> Expect.equal ( Just 5, True, False )
            , test "a description saved trimmed leaves nothing unsaved" <|
                \_ ->
                    loaded [ 1 ]
                        |> step (SetAlt 1 "Hagen i juni ")
                        |> step (SavePhoto 1)
                        |> step (PhotoSaved 1 (Ok (Fixture.photoWith { id = 1, width = 1300, height = 900, alt = "Hagen i juni", photographer = Nothing, state = "ready" })))
                        |> Gallery.isDirty
                        |> Expect.equal False
            ]
        , test "a deleted photo is no longer followed" <|
            \_ ->
                Gallery.init
                    |> Tuple.first
                    |> step (Loaded (Ok [ Fixture.photo 1, Fixture.photoWith { id = 2, width = 1300, height = 900, alt = "", photographer = Nothing, state = "preparing" } ]))
                    |> step (AskDelete 2)
                    |> step ConfirmDelete
                    |> step (Deleted 2 (Ok ()))
                    |> (\m -> ( m.order, Photos.get 2 m.photos ))
                    |> Expect.equal ( [ 1 ], Nothing )
        ]
