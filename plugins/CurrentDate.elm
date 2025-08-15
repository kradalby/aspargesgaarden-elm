module CurrentDate exposing (currentDate)

import DataSource exposing (DataSource)

-- This file is replaced at build time with the actual date
-- For local development, we use a fixed date
currentDate : DataSource String
currentDate =
    DataSource.succeed "2025-08-31"
