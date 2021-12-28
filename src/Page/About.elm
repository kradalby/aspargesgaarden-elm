module Page.About exposing (Data, Model, Msg, page)

import Css
import DataSource exposing (DataSource)
import Head
import Head.Seo as Seo
import Html.Styled exposing (..)
import Html.Styled.Attributes exposing (alt, attribute, css, for, href, id, src, type_)
import Page exposing (Page, PageWithState, StaticPayload)
import Pages.PageUrl exposing (PageUrl)
import Pages.Url
import Shared
import String.Format
import Tailwind.Breakpoints as Bp
import Tailwind.Utilities as Tw
import View exposing (View)
import View.Misc exposing (contact, container, headline, paragraph)


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


view :
    Maybe PageUrl
    -> Shared.Model
    -> StaticPayload Data RouteParams
    -> View Msg
view maybeUrl sharedModel static =
    View.html "Om oss"
        [ container
            [ div [] [ headline "Om oss" ]
            , div
                [ css
                    [ Tw.flex
                    , Tw.flex_row
                    , Tw.flex_wrap
                    ]
                ]
                [ div
                    [ css
                        [ Bp.md [ Tw.w_1over2, Tw.px_0 ]
                        , Tw.w_full
                        , Tw.px_5
                        , Tw.text_brown_text
                        ]
                    ]
                    [ paragraph "Aspargesgården byr på sjarmerende og unike lokaler i landlige omgivelser, ved kysten i Vestfold."
                    , paragraph "Lokalene har bar, scene, skjermer, musikkanlegg og et stort kjøkken med alt av utstyr. I tillegg alt av duker, bestikk, servise og pynt."
                    , paragraph "Bondens Hage har dekorert de vakre blomstene til bildene. Vi formidler gjerne kontakt til profesjonelle kokker og catering."
                    , contact
                    ]
                , div
                    [ css
                        [ Bp.md [ Tw.w_1over2, Tw.neg_mt_12 ]
                        , Tw.w_full
                        , Tw.flex
                        , Tw.flex_row
                        , Tw.flex_wrap
                        , Tw.justify_center
                        ]
                    ]
                  <|
                    List.map eventTypeIcon
                        [ "bryllup"
                        , "bursdag"
                        , "feiring"
                        , "nyttaar"
                        , "julebord"
                        , "daap"
                        , "konfirmasjon"
                        , "vitnemaal"
                        , "konferanse"
                        , "minnestund"
                        ]
                ]
            ]
        ]


eventTypeIcon : String -> Html msg
eventTypeIcon eventType =
    let
        imgSource =
            "/ikoner/{{ eventType }}.svg" |> String.Format.namedValue "eventType" eventType
    in
    img [ src imgSource, alt eventType ] []
