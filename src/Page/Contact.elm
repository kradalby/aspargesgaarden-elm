module Page.Contact exposing (Data, Model, Msg, page)

import Css
import DataSource exposing (DataSource)
import Head
import Head.Seo as Seo
import Html.Styled exposing (..)
import Html.Styled.Attributes exposing (alt, attribute, css, src)
import Page exposing (Page, StaticPayload)
import Pages.PageUrl exposing (PageUrl)
import Pages.Url
import Shared
import String.Format
import Tailwind.Breakpoints as Bp
import Tailwind.Utilities as Tw
import View exposing (View)
import View.Misc exposing (container, headline)


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


type alias Data =
    ()


data : DataSource Data
data =
    DataSource.succeed ()


head :
    StaticPayload Data RouteParams
    -> List Head.Tag
head _ =
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


view :
    Maybe PageUrl
    -> Shared.Model
    -> StaticPayload Data RouteParams
    -> View Msg
view _ _ _ =
    View.html "Kontakt oss"
        [ container
            [ div [] [ headline "Kontakt oss" ]
            , div
                [ css
                    [ Tw.flex
                    , Tw.flex_row
                    , Tw.flex_wrap
                    ]
                ]
                [ div
                    [ css
                        [ Bp.md [ Tw.w_1over2 ]
                        , Tw.w_full
                        , Tw.flex
                        , Tw.flex_col
                        , Tw.flex_wrap
                        ]
                    ]
                    [ div [ css [ Bp.md [ Tw.pt_2 ] ] ]
                        [ infoBullet "tlf" "+47 90 91 98 12"
                        , infoBullet "mail" "kristine (at) aspargesgaarden (dot) no"
                        , infoBullet "location" "Østbyveien 75, 3280 Tjodalyng"
                        ]
                    ]
                , div
                    [ css
                        [ Bp.md [ Tw.w_1over2 ], Tw.w_full ]
                    ]
                    [ iframe
                        [ css
                            [ Tw.p_2
                            , Tw.pt_4
                            , Tw.h_96
                            , Tw.w_full
                            ]
                        , src "https://calendar.google.com/calendar/embed?src=klatrerosen.no_hgf7kiksq8hrmilogq5nd6f4rs%40group.calendar.google.com&ctz=Europe%2FOslo"
                        , attribute "frameborder" ""
                        ]
                        []
                    ]
                ]
            ]
        ]


infoBullet : String -> String -> Html msg
infoBullet icon content =
    let
        imgSource =
            "/ressurser/{{ icon }}.svg" |> String.Format.namedValue "icon" icon
    in
    div [ css [ Bp.md [ Tw.pl_0 ], Tw.p_2 ] ]
        [ img
            [ css
                [ Tw.w_12
                , Tw.pr_2
                , Tw.inline_block
                ]
            , src imgSource
            , alt icon
            ]
            []
        , p
            [ css
                [ Tw.inline_block
                , Tw.text_brown
                , Tw.text_lg
                , Css.fontFamilies [ "rift-soft", "sans-serif" ]
                ]
            ]
            [ text content ]
        ]
