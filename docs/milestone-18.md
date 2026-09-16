# Milestone 18: configurable example reservation price

Partner intake has a USD price field (default $0.01, maximum $1,000.00 matching the API). Decimal dollars convert through integer cents into micros. Invalid, zero, negative, over-limit and fractional-cent values are rejected before requests. Campaign selection respects the requested amount and confirmation shows the actual reserved price.

Same-price reloads reuse an unsent hold; changing price creates a new hold. Existing holds are never repriced or silently released. The UI explains this. While a reservation response is uncertain, price editing is disabled so retry retains the same request identity and amount. The draft price survives in-page navigation.

Validation: browser regression verified six invalid inputs without reservation requests, exact $0.29 storage, same-price reuse, a new $0.37 hold, navigation persistence and settlement for exactly 370,000 micros. Existing six-scenario regression passed including lost-response recovery. Browser screenshot reviewed and no JavaScript errors. Go build and tests passed.
