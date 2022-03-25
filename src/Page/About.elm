module Page.About exposing (Data, Model, Msg, page)

import DataSource exposing (DataSource)
import Head
import Head.Seo as Seo
import Html.Styled exposing (..)
import Html.Styled.Attributes exposing (alt, css, src)
import List
import Markdown.Parser
import Markdown.Renderer
import Page exposing (Page, StaticPayload)
import Pages.PageUrl exposing (PageUrl)
import Pages.Url
import Shared
import Site exposing (commonSeo)
import String.Format
import Tailwind.Breakpoints as Bp
import Tailwind.Utilities as Tw
import TailwindMarkdownRenderer
import View exposing (View)
import View.Misc exposing (contact, container, headline)


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
    Seo.summaryLarge
        { commonSeo
            | title = "Aspargesgården | Om Oss"
        }
        |> Seo.website


view :
    Maybe PageUrl
    -> Shared.Model
    -> StaticPayload Data RouteParams
    -> View Msg
view _ _ _ =
    let
        markdown =
            """
Aspargesgården byr på sjarmerende og unike lokaler i landlige omgivelser, ved kysten i Vestfold.

Vi byr på annerledes og stemningsfylte selskapslokaler, hvor låvebryllup er vår spesialitet. Vi ønsker velkommen til alle typer selskap; bryllup, konfirmasjoner, dåp, minnestunder, bursdager, firmasamlinger, kurs, konserter og konferanser.

Låven og fjøset rommer det dere trenger av moderne teknisk utstyr, samt komfortable fasiliteter og fremkommelighet for alle! Vi har det du trenger til å dekke et vakkert festbord og skape den rette stemning!

Her hos oss kan du velge om du vil gjøre litt- mye- eller ingenting selv!

Ta en nærmere kikk på [muligheter om hva vi kan tilby](/facilities) - og skulle du ha spesielle ønsker; ikke nøl med å ta kontakt! La oss skape den perfekte rammen for ditt arrangement!


# Vertskapet

I 2014 flyttet vi fra Sandefjord til idylliske Tjølling og Aspargesgården. Tidligere eiere dyrket asparges her, og gjorde om fjøset til selskapslokaler, derav navnet «Aspargesgården».

Vi -  min mann Anders og jeg, trengte låven til kurs, møter og lager for vår salgsstyrke og Tupperware produkter.

Vi har tre voksne barn, og lille Angel, vår søte Bichon Havanais.

Jeg har alltid hatt en drøm om å bo på gård, selv om jeg er en skikkelig «byjente». Elsker gamle hus, historie, interiør, hage og å gjøre det vakkert rundt meg. Anders er «handyman» som restaurerer og bygger, og sammen utvikler vi gården kontinuerlig.

Det er en utrolig glede og skape så mange fantastiske øyeblikk, muligheter og opplevelser.

Vi ser frem til å ønske mange flere brudepar, selskap, og alle andre som har lyst til å gjøre noe morsomt her på gården velkommen!

Alt godt, Kristine & Anders
        """

        deadEndsToString deadEnds =
            deadEnds
                |> List.map Markdown.Parser.deadEndToString
                |> String.join "\n"

        content =
            markdown
                |> Markdown.Parser.parse
                |> Result.mapError deadEndsToString
                |> Result.andThen (\ast -> Markdown.Renderer.render TailwindMarkdownRenderer.renderer ast)
                |> Result.withDefault [ text "Kunne ikke laste innhold..." ]
    in
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
                  <|
                    content
                        ++ [ contact
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
            "/ressurser/{{ eventType }}.svg" |> String.Format.namedValue "eventType" eventType
    in
    img [ src imgSource, alt eventType ] []
