# kbncontent

This is a Go module which contains logic for analyzing Kibana content.

It provides a single source of truth for what types of Kibana assets are legacy and should be removed from integrations.

It was created in order to facilitate a legacy visualization validation rule in [package-spec](https://github.com/elastic/package-spec) and also powers [the script](https://github.com/elastic/visualizations_integrations_tools/blob/master/index.go) that populates the Kibana team's integration tracking dashboards.
