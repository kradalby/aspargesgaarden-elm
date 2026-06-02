module CurrentDate exposing (currentDate)

import DataSource exposing (DataSource)



-- This file is replaced at build time with the actual date (see flake.nix).
-- For local development, update this to roughly the current date.
-- The isPublished filter uses this to determine which events are visible.


currentDate : DataSource String
currentDate =
    DataSource.succeed "2026-03-30"
