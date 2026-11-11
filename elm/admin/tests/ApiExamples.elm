module ApiExamples exposing (event, eventInput, photo)

{-| The JSON examples of docs/admin-api.md, which the app must decode. The
server's tests write this file: `UPDATE_GOLDEN=1 go test ./server`.
-}


event : String
event =
    """{"id":7,"status":"draft","slug":"julemarked","slugFrozen":false,"slugFromTitle":true,"year":2026,"path":"/arrangementer/2026/julemarked","url":"https://aspargesgaarden.no/arrangementer/2026/julemarked","dateText":"14. november kl. 12.00 – 15. november 2026 kl. 16.00 på Aspargesgården","isOver":false,"title":"Julemarked 2026","summary":"Velkommen til årets julemarked i Låven.","body":"## Når og hvor\\n\\n14. og 15. november i Låven.","start":"2026-11-14","startTime":"12:00","end":"2026-11-15","endTime":"16:00","extraDates":[],"priceText":"","ticketUrl":"","showInMenu":true,"menuLabel":"Julemarked","cancelled":false,"mainPhoto":{"id":12,"width":4032,"height":3024,"alt":"Låven pyntet til jul","caption":"","photographer":{"id":3,"name":"Ola Nordmann","url":"https://example.no"},"previewUrl":"/img/v<pipeline>/<sha256>/full-<crop>-1200.jpg","thumbUrl":"/img/v<pipeline>/<sha256>/full-<crop>-480.jpg","crops":{"card":{"x":1012,"y":0,"w":2419,"h":3024},"banner":{"x":0,"y":378,"w":4032,"h":2268},"share":{"x":0,"y":453,"w":4032,"h":2117}},"quality":{"card":{"minWidth":480,"sharpWidth":768,"soft":false},"banner":{"minWidth":800,"sharpWidth":896,"soft":false},"share":{"minWidth":1200,"sharpWidth":1200,"soft":false},"full":{"minWidth":480,"sharpWidth":992,"soft":false}},"status":{"state":"preparing","done":12,"total":46,"previewReady":true,"reason":""},"updatedAt":"2026-11-10T09:12:44Z"},"extraPhotos":[],"poster":null,"publishedAt":null,"createdAt":"2026-11-01T10:00:00Z","updatedAt":"2026-11-10T09:12:44Z"}"""


eventInput : String
eventInput =
    """{"updatedAt":"2026-11-10T09:12:44Z","slug":"","title":"Julemarked 2026","summary":"","body":"","start":"2026-11-14","startTime":"12:00","end":"","endTime":"","extraDates":[],"priceText":"","ticketUrl":"","showInMenu":false,"menuLabel":"","cancelled":false,"mainPhoto":{"id":12,"alt":"Låven pyntet til jul","caption":"","photographerId":3,"updatedAt":"2026-11-10T09:12:44Z"},"extraPhotos":[{"id":13,"alt":"Boder i fjøset","caption":"Fjøset fylt av boder","photographerId":null,"updatedAt":"2026-11-09T17:02:10Z"}],"poster":null}"""


photo : String
photo =
    """{"id":12,"width":4032,"height":3024,"alt":"Låven pyntet til jul","caption":"","photographer":{"id":3,"name":"Ola Nordmann","url":"https://example.no"},"previewUrl":"/img/v<pipeline>/<sha256>/full-<crop>-1200.jpg","thumbUrl":"/img/v<pipeline>/<sha256>/full-<crop>-480.jpg","crops":{"card":{"x":1012,"y":0,"w":2419,"h":3024},"banner":{"x":0,"y":378,"w":4032,"h":2268},"share":{"x":0,"y":453,"w":4032,"h":2117}},"quality":{"card":{"minWidth":480,"sharpWidth":768,"soft":false},"banner":{"minWidth":800,"sharpWidth":896,"soft":false},"share":{"minWidth":1200,"sharpWidth":1200,"soft":false},"full":{"minWidth":480,"sharpWidth":992,"soft":false}},"status":{"state":"preparing","done":12,"total":46,"previewReady":true,"reason":""},"updatedAt":"2026-11-10T09:12:44Z"}"""
