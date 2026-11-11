module Text exposing
    ( address
    , askForOriginal
    , atFloor
    , captionHelp
    , captionLabel
    , cropTooSmall
    , error
    , movedTo
    , noAccess
    , noCrop
    , photoConflict
    , photoConflictChoice
    , photoConflictTitle
    , photoFailure
    , problem
    , sharp
    , shownSmaller
    , smallPoster
    , softCentred
    , softCrop
    , softKind
    , tookTheirs
    , uploadFailure
    , withoutScheme
    )

{-| Every message the user reads about something that went wrong, or about
how sharp a photo will be, in plain bokmål. The server sends codes and
numbers only (docs/admin-api.md), so this is the one place the wording
lives.
-}

import Api exposing (Problem)


problem : Problem -> String
problem p =
    case ( p.code, p.field ) of
        ( "title_required", _ ) ->
            "Skriv en tittel."

        ( "start_required", _ ) ->
            "Velg dagen arrangementet starter."

        ( "invalid_date", _ ) ->
            "Datoen er ikke gyldig. Velg den i kalenderen."

        ( "invalid_time", _ ) ->
            "Skriv klokkeslettet som timer og minutter, for eksempel 18:30."

        ( "slug_invalid", _ ) ->
            "Adressen kan bare ha små bokstaver uten æ, ø og å, tall og bindestrek, og kan ikke bare være tall."

        ( "slug_reserved", _ ) ->
            "Denne adressen brukes av en fast side på nettsiden. Velg en annen."

        ( "slug_taken", _ ) ->
            "Et annet arrangement samme år har allerede denne adressen. Endre den litt, for eksempel ved å legge til et ord."

        ( "slug_frozen", _ ) ->
            "Adressen kan ikke endres etter at arrangementet er publisert, så lenker som er delt fortsatt virker."

        ( "end_before_start", Just "endTime" ) ->
            "Arrangementet kan ikke slutte før det starter. Klokkeslettene gjelder hver dag, så sluttiden må være etter starttiden."

        ( "end_before_start", _ ) ->
            "Siste dag kan ikke være før første dag."

        ( "extra_date", _ ) ->
            "Ekstra datoer må komme etter første dag, og hver dato bare én gang."

        ( "ticket_url", _ ) ->
            "Lenken må være en hel nettadresse som starter med https://. Kopier den fra nettleseren og lim den inn."

        ( "main_photo_required", _ ) ->
            "Velg et bilde. Det vises i listen, øverst på siden og når noen deler lenken."

        ( "alt_required", _ ) ->
            "Beskriv bildet med noen ord."

        ( "duplicate_photo", _ ) ->
            "Det samme bildet er brukt to ganger. Ta bort det ene."

        ( "not_found", _ ) ->
            "Fant det ikke. Kanskje noen andre har slettet det."

        ( "photographer_name_required", _ ) ->
            "Skriv navnet til fotografen."

        ( "photographer_url", _ ) ->
            "Lenken må være en hel nettadresse som starter med https://."

        ( "photographer_name_taken", _ ) ->
            "Det finnes allerede en fotograf med dette navnet."

        ( "crop_too_small", _ ) ->
            cropTooSmall

        ( "crop_outside", _ ) ->
            "Rammen går utenfor bildet. Dra den litt innover."

        ( "crop_kind", _ ) ->
            "Denne beskjæringen finnes ikke. Trykk «Avbryt» og prøv igjen."

        ( "bad_request", _ ) ->
            "Noe gikk galt mellom administrasjonen og serveren. Prøv igjen. Hjelper ikke det, si fra til Kristoffer."

        ( "crop_aspect", _ ) ->
            "Rammen har feil form. Trykk «Avbryt» og prøv igjen."

        ( "conflict", _ ) ->
            "Noen andre har lagret endringer her mens du jobbet."

        ( "forbidden", _ ) ->
            noAccess

        ( "wrong_host", _ ) ->
            "Administrasjonen må åpnes fra sin egen adresse. Bruk lenken du har fått, ikke en kopi av siden."

        ( "cross_origin", _ ) ->
            "Endringen kom ikke fra administrasjonen selv, så den ble stoppet. Last inn siden på nytt og prøv igjen."

        _ ->
            uploadFailure p.code


{-| Why an upload did not go through, and what to do about it.
-}
uploadFailure : String -> String
uploadFailure code =
    case code of
        "too_large" ->
            "Filen er for stor (over 40 MB). Velg en mindre utgave av bildet."

        "unsupported_type" ->
            "Dette er ikke et bilde nettsiden kan bruke. Velg et vanlig foto, slik mobilen eller kameraet lagrer det."

        "damaged" ->
            "Filen ser ut til å være ødelagt. Velg bildet på nytt, eller velg et annet."

        "too_small" ->
            "Bildet er for lite (under 480 punkter bredt). Velg et større bilde."

        "too_many_pixels" ->
            "Bildet har for mange punkter til at nettsiden kan behandle det (over 100 millioner). Lagre en mindre kopi av bildet, og last opp den."

        _ ->
            "Noe gikk galt på serveren. Prøv igjen om litt."


{-| How sharp a photo will be on the site. Below a kind's sharp width the
browser stretches it on every screen; the server takes it all the same.
-}
sharp : String
sharp =
    "Skarpt."


softCrop : String
softCrop =
    "Dette utsnittet blir uskarpt på nettsiden. Gjør rammen større, eller bruk et større bilde om du har det."


cropTooSmall : String
cropTooSmall =
    "Utsnittet er for lite. Gjør rammen større."


{-| Why "Mindre" stops: the server's minimum for the kind.
-}
atFloor : String
atFloor =
    "Rammen kan ikke bli mindre, for nettsiden trenger minst så mye av bildet."


{-| Beside the photo in the form, for a kind whose crop is soft.
-}
softKind : String -> String
softKind usedFor =
    usedFor ++ ": utsnittet blir uskarpt på nettsiden. Trykk «Beskjær» og gjør rammen større, eller bruk et større bilde om du har det."


{-| What a photo too small for a crop kind gets instead. minWidth is the
server's minimum for the kind.
-}
noCrop : String -> Int -> String
noCrop key minWidth =
    let
        under =
            "under " ++ String.fromInt minWidth ++ " punkter bredt"
    in
    case key of
        "share" ->
            "Bildet er " ++ under ++ ", så det får ikke eget bilde for deling på Facebook. Da brukes bildet fra toppen av siden. Det går fint."

        "banner" ->
            "Bildet er for lite til toppen av siden (" ++ under ++ "), så der brukes et utsnitt fra midten."

        _ ->
            "Bildet er for lite til listen (" ++ under ++ "), så der brukes et utsnitt fra midten."


{-| Follows noCrop when the centred crop shown there is soft.
-}
softCentred : String
softCentred =
    "Der blir det uskarpt. Bruk et større bilde om du har det."


{-| With every warning about softness: the usual cause, and the cure.
-}
askForOriginal : String
askForOriginal =
    "Bilder sendt på Messenger eller e-post blir ofte gjort mindre. Be om originalen."


{-| A gallery or extra photo narrower than the lightbox: it is not
stretched, only shown smaller, so this is information, not a warning.
-}
shownSmaller : String
shownSmaller =
    "Bildet er litt lite, så det vises mindre når man åpner det stort. Det går fint."


{-| A poster narrower than the widest it is drawn: shown whole all the same.
-}
smallPoster : String
smallPoster =
    "Plakaten er litt liten, men den vises hel. Last opp en større fil hvis du har en. Da blir teksten lettere å lese."


{-| The caption beside the description: she must not write the same in
both, so the help says which one the visitors see.
-}
captionLabel : String
captionLabel =
    "Bildetekst (valgfritt)"


captionHelp : String
captionHelp =
    "Skriv en kort tekst som alle kan lese under bildet. Teksten vises når bildet åpnes stort. Beskrivelsen over vises ikke. Den er for blinde og for Google."


{-| Said aloud after a move button, with the item's new place.
-}
movedTo : Int -> String
movedTo place =
    "Bildet er flyttet til plass " ++ String.fromInt place ++ "."


{-| For someone the tailnet does not let into the admin.
-}
noAccess : String
noAccess =
    "Du har ikke tilgang til administrasjonen. Be Kristoffer om å gi deg tilgang."


{-| Why the server gave up preparing a photo's sizes.
-}
photoFailure : String -> String
photoFailure reason =
    case reason of
        "damaged" ->
            "Bildet kunne ikke leses. Filen er nok ødelagt. Ta det bort og velg et annet."

        "too_small" ->
            "Bildet er for lite til å brukes. Ta det bort og velg et større."

        "missing_original" ->
            "Den opprinnelige filen til bildet er borte fra serveren. Last opp det samme bildet på nytt."

        "unsupported_type" ->
            uploadFailure reason

        "too_large" ->
            uploadFailure reason

        "too_many_pixels" ->
            uploadFailure reason

        _ ->
            "Bildet kunne ikke klargjøres. Trykk «Prøv igjen»."


{-| A whole request failing. Says plainly that nothing typed is lost, since
the form keeps everything.
-}
error : Api.Error -> String
error e =
    case e of
        Api.Network ->
            "Fikk ikke kontakt med serveren. Sjekk at du er på nett, og prøv igjen. Det du har skrevet er ikke borte."

        Api.Problems ps ->
            String.join " " (List.map problem ps)

        Api.Conflict _ _ ->
            "Noen andre har lagret endringer her mens du jobbet."

        Api.PhotoInUse _ ->
            "Bildet er i bruk."

        Api.NameTaken _ ->
            "Det finnes allerede en fotograf med dette navnet."

        Api.Unexpected ->
            "Noe gikk galt på serveren. Prøv igjen om litt. Det du har skrevet er ikke borte."


{-| An address as she would type it: without <https://>.
-}
withoutScheme : String -> String
withoutScheme url =
    if String.startsWith "https://" url then
        String.dropLeft 8 url

    else if String.startsWith "http://" url then
        String.dropLeft 7 url

    else
        url


{-| After «Hent deres versjon».
-}
tookTheirs : String
tookTheirs =
    "Den nyeste versjonen er hentet."


photoConflictTitle : String
photoConflictTitle =
    "Noen andre har endret et bilde"


{-| alts: the descriptions of the photos someone else changed, as stored.
-}
photoConflict : List String -> String
photoConflict alts =
    let
        which =
            case List.filter ((/=) "") alts of
                [ alt ] ->
                    "bildet «" ++ alt ++ "»"

                _ ->
                    "et av bildene her"
    in
    "Mens du skrev, har noen andre endret beskrivelsen eller fotografen til "
        ++ which
        ++ ". Bildet brukes også et annet sted. Det du har skrevet er ikke borte."


photoConflictChoice : String
photoConflictChoice =
    "Vil du bruke din beskrivelse og fotograf, eller deres? Resten av det du har skrevet lagres uansett."


{-| Under the address field. site is where the public site is, slug what
she typed (empty: made from the title), path the address as last saved
("" while there is none), titleSaved whether the title is as saved, so
path is still true.
-}
address : { site : String, year : String, slug : String, path : String, titleSaved : Bool } -> String
address a =
    if a.slug == "" then
        if a.path /= "" && a.titleSaved then
            "Lages av tittelen: " ++ withoutScheme a.site ++ a.path ++ ". Den følger tittelen til arrangementet publiseres. Vil du ha en annen, skriv den her."

        else
            "Lages av tittelen når du lagrer, og følger tittelen til arrangementet publiseres. Vil du ha en annen, skriv den her."

    else
        withoutScheme a.site
            ++ "/arrangementer/"
            ++ (if a.year == "" then
                    "<år>"

                else
                    a.year
               )
            ++ "/"
            ++ a.slug
            ++ " – tøm feltet for å lage den av tittelen igjen. Kan ikke endres etter publisering."
