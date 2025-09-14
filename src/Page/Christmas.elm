module Page.Christmas exposing (Data, Model, Msg, page)

import Css
import DataSource exposing (DataSource)
import Head
import Head.Seo as Seo
import Html.Styled exposing (..)
import Html.Styled.Attributes exposing (css, src)
import Page exposing (Page, StaticPayload)
import Pages.PageUrl exposing (PageUrl)
import Pages.Url
import Shared
import Site exposing (commonSeo)
import Tailwind.Breakpoints as Bp
import Tailwind.Utilities as Tw
import View exposing (View)
import View.Misc exposing (container, headline, responsiveImg)


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
    {}


data : DataSource Data
data =
    DataSource.succeed {}


head :
    StaticPayload Data RouteParams
    -> List Head.Tag
head _ =
    Seo.summaryLarge
        { commonSeo
            | title = "Aspargesgården | Julemarked 15. & 16. november"
            , description = "Velkommen til Julemarked på Aspargesgården! Bli med oss 15. & 16. november for en helg fylt med kreativitet og julestemning."
        }
        |> Seo.website


view :
    Maybe PageUrl
    -> Shared.Model
    -> StaticPayload Data RouteParams
    -> View Msg
view _ _ _ =
    View.html "Julemarked"
        [ container
            [ div [] [ headline "Julemarked" ] ]
        , container
            [ div
                [ css
                    [ Tw.max_w_4xl
                    , Tw.mx_auto
                    , Tw.py_8
                    ]
                ]
                [ h2
                    [ css
                        [ Tw.text_4xl
                        , Tw.mb_6
                        , Tw.text_tre
                        , Css.fontFamilies [ "Cardillac", "cursive" ]
                        ]
                    ]
                    [ text "Velkommen til Julemarked på Aspargesgården!" ]
                , p
                    [ css
                        [ Tw.pb_3
                        , Tw.text_sort
                        , Css.fontFamilies [ "Avenir", "sans-serif" ]
                        ]
                    ]
                    [ text "Bli med oss på en deilig helt den 15. & 16. November, fra kl. 12 til 16, når vi forvandler Aspargesgården og skaper herlig jul. Sammen med Sandefjord Soroptimistklubb har vi gleden av å presentere et marked fylt med kreativitet og julestemning!" ]
                , p
                    [ css
                        [ Tw.pb_3
                        , Tw.text_sort
                        , Css.fontFamilies [ "Avenir", "sans-serif" ]
                        ]
                    ]
                    [ text "Her vil du finne mer enn 30 dyktige gründere som tilbyr en rekke flotte brukskunst og kortreiste delikatesser. Her finner du den ekstra fine gaven, hjemmelagde spiselige gaver til deg selv eller noen du er glad i." ]
                , p
                    [ css
                        [ Tw.pb_3
                        , Tw.text_sort
                        , Css.fontFamilies [ "Avenir", "sans-serif" ]
                        ]
                    ]
                    [ text "Ta deg tid til å nyte den nydelige kafeen vår, hvor hjemmebakst garantert vil smake." ]
                , div
                    [ css [ Tw.my_8 ] ]
                    [ h3
                        [ css
                            [ Tw.text_3xl
                            , Tw.mb_4
                            , Tw.text_tre
                            , Css.fontFamilies [ "Cardillac", "cursive" ]
                            ]
                        ]
                        [ text "Når og hvor" ]
                    , p
                        [ css [ Tw.pb_3, Tw.text_sort, Css.fontFamilies [ "Avenir", "sans-serif" ] ] ]
                        [ strong [ css [ Tw.font_bold ] ] [ text "Dato: " ]
                        , text "15. - 16. november 2025"
                        ]
                    , p
                        [ css [ Tw.pb_3, Tw.text_sort, Css.fontFamilies [ "Avenir", "sans-serif" ] ] ]
                        [ strong [ css [ Tw.font_bold ] ] [ text "Åpningstider: " ]
                        , text "Lørdag og søndag kl. 12:00 - 16:00"
                        ]
                    ]
                , div
                    [ css [ Tw.my_8 ] ]
                    [ h3
                        [ css
                            [ Tw.text_3xl
                            , Tw.mb_4
                            , Tw.text_tre
                            , Css.fontFamilies [ "Cardillac", "cursive" ]
                            ]
                        ]
                        [ text "Høydepunkter" ]
                    , ul
                        [ css [ Tw.list_disc, Tw.list_inside, Tw.space_y_2, Css.fontFamilies [ "Avenir", "sans-serif" ] ] ]
                        [ li [] [ text "Mer enn 30 dyktige gründere med brukskunst og kortreiste delikatesser" ]
                        , li [] [ text "Nydelig kafe med hjemmebakst" ]
                        , li [] [ text "Julepyntet gård med koselig bålfat ute" ]
                        , li [] [ text "Tjølling Kirkes barnekor BiTs synger julesanger søndag ca. kl. 13" ]
                        , li [] [ text "Den ekstra fine gaven eller hjemmelagde spiselige gaver" ]
                        ]
                    , p
                        [ css
                            [ Tw.mt_4
                            , Tw.text_sort
                            , Css.fontFamilies [ "Avenir", "sans-serif" ]
                            ]
                        ]
                        [ text "Gården vil være julepyntet, og vi har bålfat ute for å skape en koselig atmosfære." ]
                    ]
                , div
                    [ css [ Tw.my_8 ] ]
                    [ h3
                        [ css
                            [ Tw.text_3xl
                            , Tw.mb_4
                            , Tw.text_tre
                            , Css.fontFamilies [ "Cardillac", "cursive" ]
                            ]
                        ]
                        [ text "Veldedighet" ]
                    , p
                        [ css [ Tw.pb_3, Tw.text_sort, Css.fontFamilies [ "Avenir", "sans-serif" ] ] ]
                        [ text "Inntekten fra tombolaen, klubbens egen bod, og kafeteriaen vil gå til Sandefjord Soroptimistklubb, som jobber for å støtte kvinner og barn i vårt nærmiljø og i Moldova." ]
                    , p
                        [ css [ Tw.pb_3, Tw.text_sort, Css.fontFamilies [ "Avenir", "sans-serif" ] ] ]
                        [ text "Vi er utrolig takknemlig for at Aspargesgården kan være en vakker ramme for slike arrangementer og opplevelser." ]
                    ]
                , div
                    [ css
                        [ Tw.bg_brown
                        , Tw.bg_opacity_20
                        , Tw.p_6
                        , Tw.my_8
                        ]
                    ]
                    [ p
                        [ css [ Tw.text_lg, Tw.font_bold, Tw.mb_2, Css.fontFamilies [ "Avenir", "sans-serif" ] ] ]
                        [ text "Velkommen!" ]
                    , p [ css [ Tw.text_sort, Css.fontFamilies [ "Avenir", "sans-serif" ] ] ]
                        [ text "Ta med familie og venner, og bli med oss for å komme i førjuls stemning! Vi lover en helg fylt med glede, dufter og stemning. Velkommen!" ]
                    ]
                , div
                    [ css [ Tw.my_12 ] ]
                    [ div
                        [ css
                            [ Tw.grid
                            , Tw.grid_cols_2
                            , Tw.gap_4
                            , Bp.md [ Tw.gap_6 ]
                            ]
                        ]
                        [ responsiveImg
                            [ css
                                [ Tw.w_full
                                , Tw.h_auto
                                , Tw.rounded_lg
                                ]
                            ]
                            "arrangementer/julemarked1"
                        , responsiveImg
                            [ css
                                [ Tw.w_full
                                , Tw.h_auto
                                , Tw.rounded_lg
                                ]
                            ]
                            "arrangementer/julemarked3"
                        , responsiveImg
                            [ css
                                [ Tw.w_full
                                , Tw.h_auto
                                , Tw.rounded_lg
                                ]
                            ]
                            "arrangementer/julemarked2"
                        , responsiveImg
                            [ css
                                [ Tw.w_full
                                , Tw.h_auto
                                , Tw.rounded_lg
                                ]
                            ]
                            "arrangementer/julemarked4"
                        ]
                    ]
                ]
            ]
        ]
