module Ui exposing
    ( Field
    , badge
    , card
    , checkbox
    , dangerButton
    , errorList
    , errorText
    , field
    , focus
    , h1
    , h2
    , help
    , input
    , linkButton
    , liveRegion
    , modal
    , moveId
    , notice
    , primaryButton
    , progress
    , quietButton
    , secondaryButton
    , spinnerText
    , textarea
    , verdict
    )

{-| The admin's building blocks, in the site's look: Cardillac headings in
the site's brown, Avenir text, white cards on a light brown page. Class
strings are written out whole, because Tailwind finds them by scanning this
file.
-}

import Browser.Dom as Dom
import Html exposing (Attribute, Html, button, div, label, li, p, span, text, ul)
import Html.Attributes as A exposing (class)
import Html.Events exposing (onClick, preventDefaultOn)
import Json.Decode as D
import Task


h1 : String -> Html msg
h1 s =
    Html.h1 [ class "font-display text-tre text-4xl md:text-5xl leading-tight" ] [ text s ]


h2 : List (Attribute msg) -> String -> Html msg
h2 attrs s =
    Html.h2 (class "font-display text-tre text-2xl md:text-3xl leading-tight" :: attrs) [ text s ]


card : List (Attribute msg) -> List (Html msg) -> Html msg
card attrs =
    Html.section (class "bg-white border border-brown rounded-xl p-4 md:p-6 flex flex-col gap-4" :: attrs)



-- BUTTONS


buttonBase : String
buttonBase =
    "inline-flex items-center justify-center gap-2 min-h-12 px-5 rounded-lg border-2 font-bold text-base leading-tight text-center focus-visible:outline-3 focus-visible:outline-offset-2 focus-visible:outline-sort disabled:opacity-60 disabled:cursor-not-allowed"


primaryButton : List (Attribute msg) -> msg -> String -> Html msg
primaryButton attrs msg label =
    button (A.type_ "button" :: class buttonBase :: class "bg-sort border-sort text-white hover:bg-mork-eukalyptus hover:border-mork-eukalyptus" :: onClick msg :: attrs) [ text label ]


secondaryButton : List (Attribute msg) -> msg -> String -> Html msg
secondaryButton attrs msg label =
    button (A.type_ "button" :: class buttonBase :: class "bg-white border-sort text-sort hover:bg-brown/20" :: onClick msg :: attrs) [ text label ]


dangerButton : List (Attribute msg) -> msg -> String -> Html msg
dangerButton attrs msg label =
    button (A.type_ "button" :: class buttonBase :: class "bg-white border-rust text-rust hover:bg-rust/10" :: onClick msg :: attrs) [ text label ]


{-| A small text button for secondary actions inside a row.
-}
quietButton : List (Attribute msg) -> msg -> String -> Html msg
quietButton attrs msg label =
    button
        (A.type_ "button"
            :: class "inline-flex items-center min-h-11 px-3 rounded-lg font-bold underline underline-offset-4 hover:bg-brown/20 focus-visible:outline-3 focus-visible:outline-offset-2 focus-visible:outline-sort disabled:opacity-60"
            :: onClick msg
            :: attrs
        )
        [ text label ]


{-| A link that looks like a secondary button.
-}
linkButton : List (Attribute msg) -> String -> Html msg
linkButton attrs label =
    Html.a (class buttonBase :: class "bg-white border-sort text-sort hover:bg-brown/20 no-underline" :: attrs) [ text label ]



-- FIELDS


type alias Field msg =
    { id : String
    , label : String
    , help : String
    , errors : List String
    , control : List (Attribute msg) -> Html msg
    }


{-| A labelled control with optional help and error text. The error is text
with a marker, never only a red border, and the control points at it.
-}
field : Field msg -> Html msg
field f =
    let
        described =
            List.filterMap identity
                [ if f.help == "" then
                    Nothing

                  else
                    Just (f.id ++ "-hjelp")
                , if List.isEmpty f.errors then
                    Nothing

                  else
                    Just (f.id ++ "-feil")
                ]
    in
    div [ class "flex flex-col gap-1.5 min-w-0" ]
        [ label [ A.for f.id, class "font-bold text-lg" ] [ text f.label ]
        , if f.help == "" then
            text ""

          else
            help [ A.id (f.id ++ "-hjelp") ] f.help
        , f.control
            ([ A.id f.id
             , A.attribute "aria-invalid"
                (if List.isEmpty f.errors then
                    "false"

                 else
                    "true"
                )
             ]
                ++ (if List.isEmpty described then
                        []

                    else
                        [ A.attribute "aria-describedby" (String.join " " described) ]
                   )
            )
        , errorText (f.id ++ "-feil") f.errors
        ]


help : List (Attribute msg) -> String -> Html msg
help attrs s =
    p (class "text-sort/70 text-base [overflow-wrap:anywhere]" :: attrs) [ text s ]


errorText : String -> List String -> Html msg
errorText id errors =
    if List.isEmpty errors then
        text ""

    else
        div [ A.id id, class "flex gap-2 items-start text-rust font-bold" ]
            [ span [ A.attribute "aria-hidden" "true", class "shrink-0 inline-flex items-center justify-center w-6 h-6 rounded-full bg-rust text-white text-sm" ] [ text "!" ]
            , span [ class "[overflow-wrap:anywhere]" ] [ text ("Må rettes: " ++ String.join " " errors) ]
            ]


inputClass : String
inputClass =
    "w-full min-h-12 px-4 rounded-lg border-2 border-brown-text bg-white text-sort text-lg aria-invalid:border-rust aria-invalid:border-[3px] focus-visible:outline-3 focus-visible:outline-offset-1 focus-visible:outline-sort"


input : List (Attribute msg) -> List (Attribute msg) -> Html msg
input attrs extra =
    Html.input (class inputClass :: attrs ++ extra) []


textarea : List (Attribute msg) -> List (Attribute msg) -> Html msg
textarea attrs extra =
    Html.textarea (class inputClass :: class "py-3 leading-relaxed" :: attrs ++ extra) []


checkbox : { id : String, label : String, help : String, checked : Bool, onCheck : Bool -> msg } -> Html msg
checkbox c =
    div [ class "flex gap-4 items-start" ]
        [ Html.input
            [ A.type_ "checkbox"
            , A.id c.id
            , A.checked c.checked
            , Html.Events.onCheck c.onCheck
            , class "mt-1 w-7 h-7 shrink-0 accent-sort focus-visible:outline-3 focus-visible:outline-offset-2 focus-visible:outline-sort"
            , A.attribute "aria-describedby" (c.id ++ "-hjelp")
            ]
            []
        , div []
            [ label [ A.for c.id, class "font-bold text-lg" ] [ text c.label ]
            , help [ A.id (c.id ++ "-hjelp") ] c.help
            ]
        ]



-- STATUS


badge : String -> String -> Html msg
badge kind label =
    span
        [ class "inline-flex items-center px-3 py-1 rounded-full text-sm font-bold border"
        , class
            (case kind of
                "published" ->
                    "bg-lys-eukalyptus/40 border-mork-eukalyptus text-mork-eukalyptus"

                "draft" ->
                    "bg-kalk/40 border-tre text-sort"

                _ ->
                    "bg-white border-brown text-sort"
            )
        ]
        [ text label ]


{-| A message box for results and failures, with a word in front so the
kind is not carried by colour alone.
-}
notice : String -> List (Html msg) -> Html msg
notice kind content =
    div
        [ class "rounded-lg border-2 p-4 flex flex-col gap-3"
        , class
            (if kind == "error" then
                "border-rust bg-rust/10"

             else
                "border-mose bg-lys-eukalyptus/30"
            )
        ]
        content


{-| A judgement about a photo, "sharp", "soft", "info" or "error", with an
icon in front and in words, so the kind is not carried by colour alone.
-}
verdict : String -> List (Attribute msg) -> List String -> Html msg
verdict kind attrs lines =
    let
        ( icon, iconClass ) =
            case kind of
                "sharp" ->
                    ( "✓", "bg-mork-eukalyptus text-white" )

                "soft" ->
                    ( "!", "bg-tre text-white" )

                "error" ->
                    ( "!", "bg-rust text-white" )

                _ ->
                    ( "i", "border-2 border-brown-text text-brown-text" )
    in
    div (class "flex gap-2 items-start" :: A.attribute "data-verdict" kind :: attrs)
        [ span [ A.attribute "aria-hidden" "true", class "shrink-0 inline-flex items-center justify-center w-6 h-6 rounded-full text-sm font-bold", class iconClass ] [ text icon ]
        , div [ class "flex flex-col gap-1 [overflow-wrap:anywhere]", A.classList [ ( "text-rust", kind == "error" ) ] ]
            -- A warning's verdict stands out; the advice after it does not.
            (List.indexedMap (\i l -> p [ A.classList [ ( "font-bold", i == 0 && (kind == "soft" || kind == "error") ) ] ] [ text l ]) lines)
        ]


errorList : String -> String -> List ( String, Maybe msg ) -> Html msg
errorList id title items =
    div [ A.id id, A.tabindex -1, class "rounded-lg border-2 border-rust bg-rust/10 p-4 flex flex-col gap-2 focus-visible:outline-3 focus-visible:outline-sort" ]
        [ p [ class "font-bold text-lg" ] [ text title ]
        , ul [ class "list-disc pl-6 flex flex-col gap-1" ]
            (List.map
                (\( s, go ) ->
                    li []
                        [ text (s ++ " ")
                        , case go of
                            Just msg ->
                                quietButton [] msg "Gå dit"

                            Nothing ->
                                text ""
                        ]
                )
                items
            )
        ]


{-| A labelled bar. Drawn by hand because a native progress element ignores
the site's colours in some browsers; the role keeps it a progress bar.
-}
progress : String -> Int -> Int -> Html msg
progress caption done total =
    let
        pct =
            toFloat (clamp 0 total done) * 100 / toFloat (max 1 total)
    in
    div [ class "flex flex-col gap-1" ]
        [ span [] [ text caption ]
        , div
            [ A.attribute "role" "progressbar"
            , A.attribute "aria-label" caption
            , A.attribute "aria-valuemin" "0"
            , A.attribute "aria-valuemax" (String.fromInt total)
            , A.attribute "aria-valuenow" (String.fromInt done)
            , class "w-full h-3 rounded-full bg-kalk/60 overflow-hidden"
            ]
            [ div [ class "h-full bg-mose", A.style "width" (String.fromFloat pct ++ "%") ] [] ]
        ]


spinnerText : String -> Html msg
spinnerText s =
    p [ class "flex items-center gap-3 text-lg" ]
        [ span [ A.attribute "aria-hidden" "true", class "inline-block w-5 h-5 rounded-full border-3 border-brown border-t-sort animate-spin" ] []
        , text s
        ]


{-| Text read out by screen readers when it changes, without moving focus.
-}
liveRegion : String -> Html msg
liveRegion s =
    div [ A.attribute "aria-live" "polite", A.attribute "role" "status", class "sr-only" ] [ text s ]



-- FOCUS


{-| Moves focus to the element with id; done is sent either way, since a
control that is gone is no error.
-}
focus : msg -> String -> Cmd msg
focus done id =
    Task.attempt (\_ -> done) (Dom.focus id)


{-| The id of the move button that moved item i by delta, so focus can
follow the item to its new place.
-}
moveId : Int -> Int -> String
moveId i delta =
    (if delta < 0 then
        "flytt-tidligere-"

     else
        "flytt-senere-"
    )
        ++ String.fromInt i



-- DIALOG


{-| A modal dialog. asp-modal opens it with showModal(), so the page behind
is inert and focus stays inside; Escape, or the browser closing it, asks to
close through onClose, which must take it away. Put
`autofocus` on the control that should get focus first.
-}
modal : { id : String, title : String, onClose : msg, wide : Bool } -> List (Html msg) -> Html msg
modal m content =
    Html.node "asp-modal"
        []
        [ Html.node "dialog"
            [ A.id m.id
            , A.attribute "aria-labelledby" (m.id ++ "-tittel")
            , preventDefaultOn "cancel" (D.succeed ( m.onClose, True ))

            -- The browser may close it without asking (Chromium does on a
            -- second Escape); the page must then close it too.
            , Html.Events.on "close" (D.succeed m.onClose)
            , class "m-auto p-0 rounded-2xl bg-white text-sort backdrop:bg-sort/60 max-h-[calc(100dvh-1rem)] overflow-auto [scrollbar-gutter:stable]"
            , class
                (if m.wide then
                    "w-[min(68rem,calc(100vw-1rem))]"

                 else
                    "w-[min(36rem,calc(100vw-1rem))]"
                )
            ]
            [ div [ class "p-4 md:p-8 flex flex-col gap-5" ]
                (Html.h2 [ A.id (m.id ++ "-tittel"), class "font-display text-tre text-3xl leading-tight" ] [ text m.title ]
                    :: content
                )
            ]
        ]
