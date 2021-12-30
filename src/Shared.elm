module Shared exposing (Data, Model, Msg, template)

import Browser.Navigation
import Css
import Css.Global
import DataSource
import Html exposing (Html)
import Html.Styled
import Html.Styled.Attributes exposing (css)
import Pages.Flags
import Pages.PageUrl exposing (PageUrl)
import Path exposing (Path)
import Route exposing (Route)
import SharedTemplate exposing (SharedTemplate)
import Tailwind.Breakpoints as Bp
import Tailwind.Utilities as Tw exposing (globalStyles)
import View exposing (View)
import View.Footer
import View.Header


template : SharedTemplate Msg Model Data msg
template =
    { init = init
    , update = update
    , view = view
    , data = data
    , subscriptions = subscriptions
    , onPageChange = Just OnPageChange
    }


type Msg
    = OnPageChange
        { path : Path
        , query : Maybe String
        , fragment : Maybe String
        }
    | IncrementFromChild


type alias Data =
    ()


type alias Model =
    { showMobileMenu : Bool
    , counter : Int
    , navigationKey : Maybe Browser.Navigation.Key
    }


init :
    Maybe Browser.Navigation.Key
    -> Pages.Flags.Flags
    ->
        Maybe
            { path :
                { path : Path
                , query : Maybe String
                , fragment : Maybe String
                }
            , metadata : route
            , pageUrl : Maybe PageUrl
            }
    -> ( Model, Cmd Msg )
init navigationKey flags maybePagePath =
    ( { showMobileMenu = False
      , counter = 0
      , navigationKey = navigationKey
      }
    , Cmd.none
    )


update : Msg -> Model -> ( Model, Cmd Msg )
update msg model =
    case msg of
        OnPageChange _ ->
            ( { model | showMobileMenu = False }, Cmd.none )

        IncrementFromChild ->
            ( { model | counter = model.counter + 1 }, Cmd.none )


subscriptions : Path -> Model -> Sub Msg
subscriptions _ _ =
    Sub.none


data : DataSource.DataSource Data
data =
    DataSource.succeed ()


view :
    Data
    ->
        { path : Path
        , route : Maybe Route
        }
    -> Model
    -> (Msg -> msg)
    -> View msg
    -> { body : Html msg, title : String }
view tableOfContents page model toMsg pageView =
    { body =
        (View.Header.view page.path
            |> Html.Styled.map toMsg
        )
            :: pageView.body
            ++ [ View.Footer.view
                    |> Html.Styled.map toMsg
               ]
            |> List.append [ Css.Global.global globalStyles ]
            |> Html.Styled.div
                [ css
                    [ Tw.antialiased
                    , Tw.flex
                    , Tw.flex_col
                    , Tw.h_screen
                    , Tw.justify_start
                    ]
                ]
            |> Html.Styled.toUnstyled
    , title = pageView.title
    }
