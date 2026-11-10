module Page.Christmas exposing (Data, Model, Msg, page)

import DataSource
import Head
import Page exposing (Page, StaticPayload)
import Page.Christmas.Year_ as Year
import Pages.PageUrl exposing (PageUrl)
import Shared
import View exposing (View)


type alias Model =
    ()


type alias Msg =
    Never


type alias RouteParams =
    {}


{-| Same content as the current year's page; canonical points there.
-}
page : Page RouteParams Data
page =
    Page.single
        { head = head
        , data = DataSource.succeed {}
        }
        |> Page.buildNoState { view = view }


type alias Data =
    {}


head : StaticPayload Data RouteParams -> List Head.Tag
head _ =
    Year.seo (Just "https://aspargesgaarden.no/christmas/2026")


view : Maybe PageUrl -> Shared.Model -> StaticPayload Data RouteParams -> View Msg
view _ _ _ =
    Year.content
