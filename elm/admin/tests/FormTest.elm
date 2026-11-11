module FormTest exposing (suite)

import Expect
import Fixture
import Form
import Test exposing (Test, describe, test)
import Text


problem : String -> Maybe String -> Maybe Int -> { code : String, field : Maybe String, photoId : Maybe Int }
problem code field photoId =
    { code = code, field = field, photoId = photoId }


suite : Test
suite =
    describe "Form"
        [ describe "toInput"
            [ test "drops date rows left empty" <|
                \_ ->
                    Form.toInput "" { emptyForm | extraDates = [ "2026-11-21", "", "2026-11-28" ] }
                        |> .extraDates
                        |> Expect.equal [ "2026-11-21", "2026-11-28" ]
            , test "forgets the menu name when the event leaves the menu" <|
                \_ ->
                    Form.toInput "" { emptyForm | showInMenu = False, menuLabel = "Julemarked" }
                        |> .menuLabel
                        |> Expect.equal ""
            , test "carries the precondition" <|
                \_ -> Form.toInput "2026-11-10T09:12:44Z" emptyForm |> .updatedAt |> Expect.equal "2026-11-10T09:12:44Z"
            ]
        , describe "where an error goes"
            [ test "a missing description goes to that photo's field" <|
                \_ -> Form.fieldId (problem "alt_required" (Just "extraPhotos") (Just 13)) |> Expect.equal (Just "alt-13")
            , test "a late end time goes to the end time" <|
                \_ -> Form.fieldId (problem "end_before_start" (Just "endTime") Nothing) |> Expect.equal (Just "slutter")
            , test "a missing photo goes to the photo button" <|
                \_ -> Form.fieldId (problem "main_photo_required" (Just "mainPhoto") Nothing) |> Expect.equal (Just "hovedbilde")
            , test "a poster used twice goes to the poster button, its description to its field" <|
                \_ ->
                    [ Form.fieldId (problem "duplicate_photo" (Just "poster") (Just 12))
                    , Form.fieldId (problem "alt_required" (Just "poster") (Just 12))
                    ]
                        |> Expect.equal [ Just "plakat", Just "alt-12" ]
            , test "an unknown field has no control" <|
                \_ -> Form.fieldId (problem "x" (Just "nope") Nothing) |> Expect.equal Nothing
            , test "a field gets all its messages, in words" <|
                \_ ->
                    Form.errorsFor "tittel" [ problem "title_required" (Just "title") Nothing, problem "ticket_url" (Just "ticketUrl") Nothing ]
                        |> Expect.equal [ Text.problem (problem "title_required" Nothing Nothing) ]
            ]
        , describe "the address"
            [ test "one that follows the title is an empty field" <|
                \_ -> Form.fromEvent (Fixture.event { id = 4, updatedAt = "1", summary = "" }) |> .slug |> Expect.equal ""
            , test "one she chose is in the field" <|
                \_ ->
                    Fixture.event { id = 4, updatedAt = "1", summary = "" }
                        |> (\e -> Form.fromEvent { e | slugFromTitle = False })
                        |> .slug
                        |> Expect.equal "kurs"
            , test "a published one is shown as it is" <|
                \_ ->
                    Fixture.event { id = 4, updatedAt = "1", summary = "" }
                        |> (\e -> Form.fromEvent { e | slugFrozen = True })
                        |> .slug
                        |> Expect.equal "kurs"
            ]
        , describe "afterSave"
            [ test "nothing typed meanwhile: take what was stored" <|
                \_ ->
                    Form.afterSave { sent = emptyForm, current = emptyForm, stored = { emptyForm | title = "Julemarked" } }
                        |> .title
                        |> Expect.equal "Julemarked"
            , test "typing meanwhile is kept, and the address still follows the title" <|
                \_ ->
                    -- A first title saved as a draft must not fix the address:
                    -- stored, an address made from the title is an empty field.
                    Form.afterSave
                        { sent = { emptyForm | title = "Julemarkd" }
                        , current = { emptyForm | title = "Julemarked" }
                        , stored = { emptyForm | title = "Julemarkd" }
                        }
                        |> (\f -> ( f.title, f.slug ))
                        |> Expect.equal ( "Julemarked", "" )
            , test "a chosen address comes back as the server stored it" <|
                \_ ->
                    Form.afterSave
                        { sent = { emptyForm | slug = "jul " }
                        , current = { emptyForm | slug = "jul ", title = "Jul" }
                        , stored = { emptyForm | slug = "jul" }
                        }
                        |> .slug
                        |> Expect.equal "jul"
            , test "typing meanwhile answers to the photos' new versions" <|
                \_ ->
                    let
                        photo v alt =
                            Just { id = 12, alt = alt, caption = "", photographerId = Nothing, updatedAt = v }
                    in
                    Form.afterSave
                        { sent = { emptyForm | mainPhoto = photo "1" "Låven" }
                        , current = { emptyForm | mainPhoto = photo "1" "Låven i snø" }
                        , stored = { emptyForm | mainPhoto = photo "2" "Låven" }
                        }
                        |> .mainPhoto
                        |> Expect.equal (photo "2" "Låven i snø")
            , test "an address typed meanwhile is kept too" <|
                \_ ->
                    Form.afterSave
                        { sent = emptyForm
                        , current = { emptyForm | slug = "mitt" }
                        , stored = { emptyForm | slug = "jul" }
                        }
                        |> .slug
                        |> Expect.equal "mitt"
            ]
        , describe "lists"
            [ test "move swaps with the neighbour" <|
                \_ -> Form.move 1 -1 [ 'a', 'b', 'c' ] |> Expect.equal [ 'b', 'a', 'c' ]
            , test "move past the start does nothing" <|
                \_ -> Form.move 0 -1 [ 'a', 'b' ] |> Expect.equal [ 'a', 'b' ]
            , test "move past the end does nothing" <|
                \_ -> Form.move 1 1 [ 'a', 'b' ] |> Expect.equal [ 'a', 'b' ]
            , test "removeAt and setAt" <|
                \_ -> [ 'a', 'b', 'c' ] |> Form.removeAt 1 |> Form.setAt 1 'z' |> Expect.equal [ 'a', 'z' ]
            ]
        ]


emptyForm : Form.Form
emptyForm =
    Form.empty
