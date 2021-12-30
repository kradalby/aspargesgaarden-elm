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
            [ div [] [ headline "Om gården" ]
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
                    , paragraph "Vi byr på annerledes og stemningsfylte selskapslokaler, hvor vi ønsker velkommen til alle typer selskap; bryllup, konfirmasjoner, dåp, minnestunder, bursdager, firmasamlinger, kurs , konserter og konferanser."
                    , paragraph "Låven og fjøset rommer det dere trenger av moderne teknisk utstyr, samt komfortable fasiliteter og fremkommelighet for alle! Vi har det du trenger til å dekke et vakkert festbord og skape den rette stemning!"
                    , paragraph "Her hos oss kan du velge om du vil gjøre litt- mye- eller ingenting selv!"
                    , paragraph "Ta en nærmere kikk på muligheter om hva vi kan tilby - og skulle du ha spesielle ønsker; ikke nøl med å ta kontakt! La oss skape den perfekte rammen for ditt arrangement!"
                    , br [] []
                    , br [] []
                    , headline "Vertskap"
                    , paragraph "I 2014 flyttet vi fra Sandefjord til idylliske Tjølling og Aspargesgården. Tidligere eiere dyrket asparges her, og gjorde om fjøset til selskapslokaler, derav navnet «Aspargesgården»"
                    , paragraph "Vi -  min mann Anders og jeg, trengte låven til kurs, møter og lager for vår salgsstyrke og Tupperware produkter."
                    , paragraph "Vi har tre voksne barn, og lille Angel vår søte Bichon Havanais."
                    , paragraph "Jeg har alltid hatt en drøm om å bo på gård, selv om jeg er en skikkelig «byjente». Elsker gamle hus, historie, interiør, hage og å gjøre det vakkert rundt meg. Anders er «handyman» som restaurerer og bygger, og sammen utvikler vi gården kontinuerlig."
                    , paragraph "Det er en utrolig glede og skape så mange fantastiske øyeblikk, muligheter og opplevelser."
                    , paragraph "Vi ser frem til å ønske mange brudepar, selskap, og alle andre som har lyst til å gjøre noe morsomt her på gården velkommen!"
                    , paragraph "Alt godt, Kristine & Anders"
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
