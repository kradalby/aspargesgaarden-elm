module Venue exposing (name, phoneLink)

{-| Facts about the venue the admin writes into text, kept in one place. The
public site states the same facts in pages/kontakt.go.
-}


name : String
name =
    "Aspargesgården"


{-| What the "Telefonlenke" button inserts: the number as the site writes
it, linked so a tap on a phone calls it.
-}
phoneLink : String
phoneLink =
    "[90 91 98 12](tel:+4790919812)"
