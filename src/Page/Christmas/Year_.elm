module Page.Christmas.Year_ exposing (Data, Model, Msg, page)

import DataSource exposing (DataSource)
import Head
import Head.Seo as Seo
import Html.Styled exposing (..)
import Html.Styled.Attributes exposing (alt, css)
import Page exposing (Page, StaticPayload)
import Pages.PageUrl exposing (PageUrl)
import Pages.Url
import Path
import Shared
import Site exposing (commonSeo)
import Tailwind.Breakpoints as Bp
import Tailwind.Utilities as Tw
import View exposing (View)
import View.Event as Event
import View.Misc exposing (container, headline, responsiveImg)


type alias Model =
    ()


type alias Msg =
    Never


type alias RouteParams =
    { year : String }


page : Page RouteParams Data
page =
    Page.prerender
        { head = head
        , data = \_ -> DataSource.succeed {}
        , routes = DataSource.succeed [ { year = "2026" } ]
        }
        |> Page.buildNoState { view = view }


type alias Data =
    {}


head : StaticPayload Data RouteParams -> List Head.Tag
head _ =
    Seo.summaryLarge
        { commonSeo
            | title = "Aspargesgården | Julemarked 14. & 15. november 2026"
            , description = "Velkommen til julemarked på Aspargesgården 14. og 15. november 2026 kl. 12–16. Mer enn 30 utstillere, kortreiste delikatesser, hjemmebakst og julestemning."
            , image =
                { url = [ "arrangementer", "2026-11-14-julemarked.jpeg" ] |> Path.join |> Pages.Url.fromPath
                , alt = "Julemarked på Aspargesgården 14.–15. november kl. 12–16"
                , dimensions = Nothing
                , mimeType = Nothing
                }
        }
        |> Seo.website


view : Maybe PageUrl -> Shared.Model -> StaticPayload Data RouteParams -> View Msg
view _ _ _ =
    View.html "Julemarked 2026"
        [ container [ div [] [ headline "Julemarked 2026" ] ]
        , Event.content
            [ Event.introduction "Velkommen til Julemarked på Aspargesgården!"
            , Event.paragraph "Bli med oss på en herlig helg den 14. og 15. november, fra kl. 12 til 16, når vi forvandler Aspargesgården og skaper herlig jul. Vi gleder oss til å presentere et marked fylt med kreativitet og julestemning!"
            , Event.paragraph "Her vil du finne mer enn 30 dyktige gründere som tilbyr en rekke flotte brukskunst og kortreiste delikatesser. Her finner du den ekstra fine gaven, hjemmelagde spiselige gaver til deg selv eller noen du er glad i."
            , Event.paragraph "Ta deg tid til å nyte den nydelige kafeen vår, hvor hjemmebakst garantert vil smake."
            , Event.section "Når og hvor"
                [ Event.detail "Dato" "14.–15. november 2026"
                , Event.detail "Åpningstider" "Lørdag og søndag kl. 12:00–16:00"
                , Event.detail "Sted" "Aspargesgården, Østbyveien 75, 3280 Tjodalyng (Larvik)"
                ]
            , responsiveImg
                [ css [ Tw.w_full, Tw.h_auto, Tw.my_8 ]
                , alt "Julemarked på Aspargesgården lørdag og søndag 14.–15. november kl. 12–16. Mer enn 30 utstillere, kafeteria og tombola."
                ]
                "arrangementer/2026-11-14-julemarked"
            , Event.section "Høydepunkter"
                [ Event.bulletList
                    [ "Mer enn 30 dyktige gründere med brukskunst og kortreiste delikatesser"
                    , "Nydelig kafe med hjemmebakst"
                    , "Julepyntet gård med koselig bålfat ute"
                    , "Den ekstra fine gaven eller hjemmelagde spiselige gaver"
                    ]
                , p [ css [ Tw.mt_4 ] ]
                    [ text "Gården vil være julepyntet, og vi har bålfat ute for å skape en koselig atmosfære." ]
                , p [ css [ Tw.mt_4, Tw.font_bold ] ]
                    [ text "På søndagen, ca. kl. 13, vil Tjølling Kirkes barnekor BiTs synge julesanger for oss." ]
                ]
            , Event.paragraph "Vi er utrolig takknemlige for at Aspargesgården kan være en vakker ramme for slike arrangementer og opplevelser."
            , Event.callout "Velkommen!"
                [ Event.paragraph "Ta med familie og venner, og bli med oss for å komme i førjulsstemning! Vi lover en helg fylt med glede, dufter og stemning. Velkommen!"
                ]
            , div [ css [ Tw.my_12 ] ]
                [ div
                    [ css [ Tw.grid, Tw.grid_cols_2, Tw.gap_4, Bp.md [ Tw.gap_6 ] ] ]
                    (List.map
                        (\( photoPath, description ) ->
                            responsiveImg
                                [ css [ Tw.w_full, Tw.h_auto, Tw.rounded_lg ]
                                , alt description
                                ]
                                photoPath
                        )
                        [ ( "arrangementer/julemarked1", "Julestemning på Aspargesgården" )
                        , ( "arrangementer/julemarked3", "Fra et tidligere julemarked på Aspargesgården" )
                        , ( "arrangementer/julemarked2", "Et tilbakeblikk på julemarkedet" )
                        , ( "arrangementer/julemarked4", "Julemarked på gården" )
                        ]
                    )
                ]
            ]
        ]
