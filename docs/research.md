# Why this domain fits

Reviewed September 15, 2026. These are public sources and design inspiration, not a description of CCO's private implementation.

| Public source | Relevant observation | How Afterglow uses it |
|---|---|---|
| [CCO home](https://clearchanneloutdoor.com/) | Outdoor inventory, creative delivery and measurement are connected business concerns. | Show the operational path between campaign budget and delivery evidence. |
| [Programmatic advertising](https://clearchanneloutdoor.com/programmatic-advertising/) | Digital inventory is available through programmatic partners, including Vistar. | Model an asynchronous partner boundary and reconciliation. |
| [Media formats](https://clearchanneloutdoor.com/our-media/) | Airport and roadside digital formats are part of the inventory. | Use eight fictional screen records across those formats. |
| [RADAR](https://clearchanneloutdoor.com/radar-data-solutions/) | Campaign planning and measurement use aggregated/anonymous data. | Avoid personal data. Do not conflate a playback with audience measurement. |
| [Case studies](https://clearchanneloutdoor.com/case-studies/) | Advertisers care about measurable campaign outcomes. | Make operational evidence inspectable without fabricating attribution lift. |
| [Creative solutions](https://clearchanneloutdoor.com/creative-solutions/) | Creative content is a distinct part of delivering outdoor campaigns. | Keep creative optimization outside the financial receipt contract. |
| [RADAR privacy supplement](https://clearchanneloutdoor.com/radar-privacy-supplement/) | Data use and privacy are explicit concerns. | Store no device IDs, identity profiles or movement histories. |
| [Vistar partnership announcement](https://www.vistarmedia.com/news/clear-channel-outdoor-selects-vistar-media-as-dooh-technology-partner) | The September 2025 announcement covers ad serving, player/device infrastructure and airport rollout. | Focus on reservation-to-playback reliability rather than a generic dashboard. |
| [Vistar integration guide](https://help.vistarmedia.com/hc/en-us/articles/360030978251-Step-by-step-integration-guide) | Ad requests, leased ads and proof-of-play notifications have a documented lifecycle and certification process. | Use a synthetic contract; explicitly avoid claiming API compatibility or certification. |
| [Fluxgate README](https://github.com/jon-jc/fluxgate) | Durable acceptance, idempotency and asynchronous telemetry are relevant foundations. | Explore a different business problem: finite budget reservations and financial effects, not another telemetry aggregator. |

The supplied screenshots reinforced the emphasis on audience relevance, varied outdoor formats, creative work and rapid activation. They contained marketing context, not instructions to contact anyone. No separate full JD document was available beyond the role description pasted in the request.

## Technical references

- [Google Pub/Sub Go v2 client](https://docs.cloud.google.com/go/docs/reference/cloud.google.com/go/pubsub/v2/latest)
- [Google Pub/Sub emulator](https://docs.cloud.google.com/pubsub/docs/emulator)
- [Cloud Run v2 Terraform resource](https://registry.terraform.io/providers/hashicorp/google/latest/docs/resources/cloud_run_v2_service)
- [Pub/Sub subscription Terraform resource](https://registry.terraform.io/providers/hashicorp/google/latest/docs/resources/pubsub_subscription)

The relevant product pages and integration material were reviewed. This is not a claim that every page, case study, legal document and investor filing across the company's entire web presence was exhaustively read.
