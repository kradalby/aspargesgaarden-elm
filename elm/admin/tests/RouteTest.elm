module RouteTest exposing (suite)

import Expect
import Route exposing (Route(..))
import Test exposing (Test, describe, test)
import Text
import Url


parse : String -> Maybe Route
parse path =
    Url.fromString ("http://admin.test" ++ path) |> Maybe.andThen Route.fromUrl


suite : Test
suite =
    describe "Route and Text"
        [ test "every address the app links to comes back as itself" <|
            \_ ->
                [ Overview, NewEvent, EditEvent 12, Gallery ]
                    |> List.map (\r -> parse (Route.toString r))
                    |> Expect.equal [ Just Overview, Just NewEvent, Just (EditEvent 12), Just Gallery ]
        , test "with or without the trailing slash" <|
            \_ -> List.map parse [ "/admin", "/admin/", "/admin/galleri/" ] |> Expect.equal [ Just Overview, Just Overview, Just Gallery ]
        , test "unknown addresses are not guessed" <|
            \_ -> List.map parse [ "/admin/arrangementer/x", "/admin/nope", "/" ] |> Expect.equal [ Nothing, Nothing, Nothing ]

        -- Every code in docs/admin-api.md has its own words; a code that falls
        -- through to the generic text means the contract and the app drifted.
        , test "every documented error code has its own message" <|
            \_ ->
                [ "not_found", "invalid_date", "invalid_time", "slug_invalid", "slug_reserved", "slug_taken", "slug_frozen", "end_before_start", "extra_date", "ticket_url", "title_required", "start_required", "main_photo_required", "alt_required", "duplicate_photo", "crop_outside", "crop_aspect", "crop_too_small", "too_large", "unsupported_type", "damaged", "too_small", "photographer_name_required", "photographer_url", "photographer_name_taken" ]
                    |> List.filter (\code -> Text.problem { code = code, field = Nothing, photoId = Nothing } == Text.uploadFailure "")
                    |> Expect.equal []
        ]
