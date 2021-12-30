module Page.Index exposing (Data, Model, Msg, page)

import Css
import DataSource exposing (DataSource)
import Head
import Head.Seo as Seo
import Html.Styled exposing (..)
import Html.Styled.Attributes exposing (alt, attribute, css, for, href, id, src, type_)
import Page exposing (Page, StaticPayload)
import Pages.PageUrl exposing (PageUrl)
import Pages.Url
import Shared
import Tailwind.Breakpoints as Bp
import Tailwind.Utilities as Tw
import View exposing (View)
import View.Misc exposing (contact)


type alias Model =
    ()


type alias Msg =
    Never


type alias RouteParams =
    {}


page : Page RouteParams Data
page =
    Page.single
        { head = head
        , data = data
        }
        |> Page.buildNoState { view = view }


data : DataSource Data
data =
    DataSource.succeed ()


head :
    StaticPayload Data RouteParams
    -> List Head.Tag
head static =
    Seo.summary
        { canonicalUrlOverride = Nothing
        , siteName = "elm-pages"
        , image =
            { url = Pages.Url.external "TODO"
            , alt = "elm-pages logo"
            , dimensions = Nothing
            , mimeType = Nothing
            }
        , description = "TODO"
        , locale = Nothing
        , title = "TODO title" -- metadata.title -- TODO
        }
        |> Seo.website


type alias Data =
    ()


view :
    Maybe PageUrl
    -> Shared.Model
    -> StaticPayload Data RouteParams
    -> View Msg
view maybeUrl sharedModel static =
    View.html "Velkommen"
        [ div
            [ css
                [ Tw.h_screen
                , Tw.flex
                , Tw.flex_col
                , Tw.justify_center
                , Css.backgroundImage <| Css.url "/bilder/forside.jpg"
                , Css.backgroundRepeat Css.noRepeat
                , Css.backgroundPosition Css.center
                , Css.backgroundSize Css.cover
                , Css.backgroundAttachment Css.fixed
                ]
            ]
            [ div
                [ css
                    [ Bp.lg [ Tw.text_4xl ]
                    , Tw.text_center
                    , Tw.text_white
                    , Tw.text_3xl
                    , Css.fontFamilies [ "rift-soft", "sans-serif" ]
                    ]
                ]
                [ text "Velkommen til" ]
            , div
                [ css
                    [ Bp.lg [ Tw.text_8xl ]
                    , Tw.text_center
                    , Tw.text_white
                    , Tw.text_5xl
                    , Tw.py_6
                    , Css.fontFamilies [ "luxus-brut", "cursive" ]
                    ]
                ]
                [ text "Aspargesgården" ]
            , contact
            ]
        ]
