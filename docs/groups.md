# MeshCom Public Groups

MeshCom uses numeric group numbers as packet destination (`dst` field). A broadcast to `*` reaches all nodes. Group numbers route messages to regional or thematic communities.

The frontend resolves known group numbers to a human-readable label and a flag emoji derived from the HAM radio callsign prefix. Unknown group numbers are shown as plain numbers.

## Group Directory

| Group       | Prefix    | Note                          | Reach                              |
|-------------|-----------|-------------------------------|------------------------------------|
| 2           | EU        | Europa                        | Europaweit                         |
| 3           | US        | USA                           | US-weit                            |
| 9           | LOC       | Local group                   | MeshCom HF cloud only              |
| 10          | WW-GE     | Worldwide German              | Deutschsprachige Meldungen         |
| 13          | WW-EN     | Worldwide English             | Englischsprachige Meldungen        |
| 19000       | F         | France dép. 19 & 87           |                                    |
| 20          | DACH      | D-A-CH                        | Deutschland, Österreich, Schweiz   |
| 204         | PA        | Netherlands                   |                                    |
| 206         | ON        | Belgium                       |                                    |
| 208         | F         | France                        |                                    |
| 214         | EA        | Spain                         |                                    |
| 222         | I         | Italy                         |                                    |
| 22201       | I         | Lazio                         |                                    |
| 22202       | I         | Sardegna                      |                                    |
| 22203       | I         | Umbria                        |                                    |
| 22211       | I         | Liguria                       |                                    |
| 22213       | I         | Valle d'Aosta                 |                                    |
| 22221       | I         | Lombardia                     |                                    |
| 22231       | I         | Friuli Venezia Giulia         |                                    |
| 22232       | I         | Trentino Alto Adige           |                                    |
| 22233       | I         | Veneto                        |                                    |
| 22241       | I         | Emilia Romagna                |                                    |
| 22251       | I         | Toscana                       |                                    |
| 22261       | I         | Abruzzo                       |                                    |
| 22262       | I         | Marche                        |                                    |
| 22271       | I         | Puglia                        |                                    |
| 22281       | I         | Basilicata                    |                                    |
| 22282       | I         | Calabria                      |                                    |
| 22283       | I         | Campania                      |                                    |
| 22284       | I         | Molise                        |                                    |
| 22291       | I         | Sicilia                       |                                    |
| 22299       | I         | Meteo/data/sensors            |                                    |
| 226         | YO        | Romania                       |                                    |
| 228         | HB        | Switzerland                   |                                    |
| 232         | OE        | Austria                       |                                    |
| 2321–2329   | OE1–OE9   | OE Bundesländer               |                                    |
| 234         | G         | Great Britain                 |                                    |
| 238         | OZ        | Denmark                       |                                    |
| 240         | SA        | Sweden                        |                                    |
| 260         | SP        | Poland                        |                                    |
| 262         | DL        | Germany                       |                                    |
| 26200–26299 | DL00–DL99 | Germany regional              | Deutschsprachige Meldungen         |
| 2622        | DL 2      | Schleswig-Holstein            |                                    |
| 26206       | DL 06     | DARC Ortsverband Dachau C06   |                                    |
| 26207       | DL 07     | Sachsen-Anhalt                |                                    |
| 26216       | DL 16     | Chiemgau                      |                                    |
| 26220       | DL 20     | Großraum Hamburg              |                                    |
| 26221       | DL 21     | Stadt Hamburg                 |                                    |
| 26225       | DL 25     | AFU Nord                      |                                    |
| 26235       | DL 35     | NI-Südheide                   |                                    |
| 26242       | DL 42     | Münsterland                   |                                    |
| 26244       | DL 44     | Freising                      |                                    |
| 26251       | DL 51     | Rhein-Berg                    | Rheinisch-Bergischer-Kreis         |
| 26255       | DL 55     | Pfalz                         |                                    |
| 26266       | DL 66     | Saar                          |                                    |
| 26269       | DL 69     | Hessen/Rheinland Pfalz        |                                    |
| 26289       | DL 89     | München Stadt                 |                                    |
| 26295       | DL 95     | Ostthüringen                  |                                    |
| 26298       | DL 98     | Thüringen                     |                                    |
| 26379       | DL3 79    | Hochrhein                     |                                    |
| 292         | T7        | San Marino                    |                                    |
| 293         | S5        | Slovenia                      |                                    |
| 30          | SV        | Greece                        |                                    |
| 460         | B         | China                         |                                    |
| 901         | 9V        | Singapore                     |                                    |

## Prefix → Flag Mapping

| Prefix  | Country / Region            | Flag |
|---------|-----------------------------|------|
| EU      | Europe                      | 🇪🇺   |
| US      | USA                         | 🇺🇸   |
| LOC     | Local                       | 📍   |
| WW-GE   | Worldwide German            | 🌍   |
| WW-EN   | Worldwide English           | 🌎   |
| DACH    | Germany / Austria / Switzerland | 🏔️ |
| F       | France                      | 🇫🇷   |
| PA      | Netherlands                 | 🇳🇱   |
| ON      | Belgium                     | 🇧🇪   |
| EA      | Spain                       | 🇪🇸   |
| I       | Italy                       | 🇮🇹   |
| YO      | Romania                     | 🇷🇴   |
| HB      | Switzerland                 | 🇨🇭   |
| OE      | Austria                     | 🇦🇹   |
| G       | Great Britain               | 🇬🇧   |
| OZ      | Denmark                     | 🇩🇰   |
| SA      | Sweden                      | 🇸🇪   |
| SP      | Poland                      | 🇵🇱   |
| DL      | Germany                     | 🇩🇪   |
| SV      | Greece                      | 🇬🇷   |
| B       | China                       | 🇨🇳   |
| 9V      | Singapore                   | 🇸🇬   |
| T7      | San Marino                  | 🇸🇲   |
| S5      | Slovenia                    | 🇸🇮   |
