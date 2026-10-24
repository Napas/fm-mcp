# Player interest ground truth — KŽ save 2036-01-16 (data/kz-2036-01-16.fm), in game, FC Metz (human)

Player Database, conditions: Position can play D (R); age 24–25; Knowledge level at least Extensive Knowledge;
Nationality is EU National; Scouted players only off.

| Player's interest setting | Count |
|---|---|
| Not considered (= no interest filter) | 23 |
| Transfer or loan · Doubtful | 15 |
| Transfer only · Persuadable | 13 |
| Transfer only · Interested | 13 (same names as Persuadable) |

Base set (23): Maitere Wong (FC Nantes), Jean Eloi Musongi (no club, FrA), Modou Sall (Genoa), Frode Øverenget (Vålerenga),
Muzi Molefe (FCSM, RSA), Oumar Cissokho (SM Caen), Franc Ortega (Huesca, injured), Julien Barthes (SM Caen),
Halifa Guibert (Blackburn), Maxime Cipriano (Saint-Étienne), Zvonko Nešić (Red Star FC), Thierry Andreani (Havre AC),
Benjamin Masson (Troyes), Ibrahim Diallo (FC Nantes), Simone Scotti (Reims), Jérémy Catherine (Angers SCO),
Logan Lafleur (Famalicão, Wnt), Eneko Gamboa (Castellón), Unai Marco (Coruña B, Trn), Álvaro Garrido (Valencia, Unh),
Aleksei Korshun (Nacional da Madeira, BLR, Ask), Marko Tomašević (AZ, CRO), Massaër Sonko (Portland, SEN, Wnt).

Labels:
- **Transfer Interested (13)**: Wong, Musongi, Sall, Øverenget, Cissokho, Cipriano, Nešić, Masson, Diallo, Scotti, Catherine, Garrido, Korshun.
- **Transfer-or-loan Doubtful but not transfer Persuadable (2)**: Lafleur, Tomašević (transfer Doubtful, or loan only).
- **Fail Transfer-or-loan Doubtful (8)**: Molefe, Ortega, Barthes, Guibert, Andreani, Gamboa, Marco, Sonko.
- Nobody is exactly Persuadable (transfer). The result is bimodal.

Key pair: Barthes and Cissokho, both SM Caen (rep 5525, div 12), reason 2 = +6772 for both; Cissokho Interested, Barthes fails Doubtful.

Reason-2 values (Follow-up 1, cand16.txt): excluded Guibert +2116, Ortega +3072, Andreani +3716, Marco +5189, Barthes +6772,
Gamboa +11680 (Molefe, Sonko not computed). Included Scotti −3328, Cipriano −2072, Tomašević −360, others positive.
Erveodoso (GK, Nice): agent says he wouldn't be interested (separate ground truth; reason "improve their standing in the Ligue 1").

# Validation set 2 — DC (in game, 2036-01-16)

Conditions: Position can play D (C); age 24–25; Knowledge level at least Extensive; Nationality EU National; Scouted only off.

| Setting | Count |
|---|---|
| Not considered (no interest filter) | 41 |
| Transfer or loan · Persuadable | 24 |

Base set (41), in game order: Mladen Jelavić (Hajduk, Wnt), Francesco Manta (Montpellier), Gabriel Ahovey (Olympique de Marseille, BEN, Inj),
Divine Adetunji (Ajax, ENG), Murtala Ejike (R. Santander, NGA), David Silva (LUX, no club, FrA), Erwan Sueur (Brest, Inj),
Jean Eloi Musongi (no club, FrA), Romualdas Baranaus… (Šiauliai, LTU), Manuel Vega (R. Santander, URU), Jesús Blanco (OGC Nice),
Oskar Hedeager (Lyngby), Giovanni Lotito (Crotone, Wnt), Christos Heed (Sparta Rotterdam, SWE), Lander Martínez de Eu… (Alavés, Inj),
Yves Guignard (Auxerre), Théo Rougier (Portsmouth), Jakub Cichy (PSV, NED, Trn), Bubacar Cá (AVS SAD, GNB, Lst), Kwesi Danso (Gil Vicente, GHA),
Joseba Fraile (Tarazona, Ask), Eugeni Folch (LOSC), Raimundo Nonato (CUI, BRA), Eric Tanko (OL, CMR), Gallio Harmanus (De Graafschap, NED),
Mathieu Boulay (FC Nantes), Vahe Israelyan (Zlín, ARM), Thorbjørn Nødgaard (AB, DEN), Pablo Vilas (Borussia M'gladbach, ESP),
Sfiso Mncube (Toulouse FC, RSA), Ahmed Coulibaly (Toulouse FC, MLI, Inj), Marek Witkowski (Lech, POL), Demba Diédhiou (Vigo, SEN),
Gianfranco Fiaschetti (1. FSV Mainz 05, ITA), Luigi (Atromitos, ITA), Lau Jiménez (Strasbourg, ESP), Luis Adrianza (Tenerife, VEN, Inj),
Ronald van Hattum (no club, NED, Wnt), André Vuillemin (Torreense, FRA), Leonel Pessoa Bastos (A. Madrid, POR, Lst), Baye Gora Souaré (Sharjah FC, SEN).

- **Pass Transfer-or-loan Persuadable (24)**: Jelavić, Manta, Musongi, Baranaus…, Vega, Hedeager, Lotito, Lander Martínez de Eu…, Guignard, Cá, Danso,
  Fraile, Nonato, Tanko, Boulay, Israelyan, Nødgaard, Mncube, Coulibaly, Luigi, Adrianza, van Hattum, Vuillemin, Pessoa Bastos.
- **Fail Persuadable (17)** (may still be Doubtful): Ahovey, Adetunji, Ejike, David Silva, Sueur, Jesús Blanco, Heed, Rougier, Cichy (Trn), Folch,
  Harmanus, Vilas, Witkowski, Diédhiou, Fiaschetti, Lau Jiménez (Strasbourg = a Metz rival), Souaré.

## Validation set 2 — transfer only (in game, 2036-01-16, same DC conditions)

| Setting | Count |
|---|---|
| Transfer only · Persuadable | 23 |
| Transfer only · Doubtful | 23 (identical names) |

- **Pass transfer (23)**: Jelavić, Manta, Musongi, Baranaus…, Hedeager, Lotito, Lander Martínez de Eu…, Guignard, Cá, Danso, Fraile, Nonato, Tanko,
  Boulay, Israelyan, Nødgaard, Mncube, Coulibaly, Luigi, Adrianza, van Hattum, Vuillemin, Pessoa Bastos.
- = the transfer-or-loan Persuadable list minus **Manuel Vega** (R. Santander), who passes only through the loan rule.
- **Fail transfer even at Doubtful (18)**: Ahovey, Adetunji, Ejike, David Silva, Sueur, Jesús Blanco, Heed, Rougier, Cichy, Folch, Harmanus,
  Vilas, Witkowski, Diédhiou, Fiaschetti, Lau Jiménez, Souaré, Vega.
- Again bimodal: nobody is exactly Doubtful or exactly Persuadable for transfer.

# Validation set 3 — ST (in game, 2036-01-16) — held out, blind

Conditions: Position can play ST (C); age 24–25; Knowledge level at least Extensive; Nationality EU National; Scouted only off.

| Setting | Count |
|---|---|
| Not considered (no interest filter) | 31 |
| Transfer only · Persuadable | 18 |
| Transfer only · Doubtful | 18 (count seen behind the conditions dialog; names assumed identical) |

Base set (31): Mutala Keïta (Genoa, MLI), Jean-Pierre Bayal (US Thionville Lusitanos, SEN), Hani Boutaleb (Auxerre, ALG, Lst),
Mert Kaplan (Basel, SUI), Titouan Vacher (FCSM, FRA), Kim Forsberg (Malmö FF, SWE), Mauricio Jiménez (Coruña, COL),
Dariusz Zbiciak (FC St. Pauli, GER, Wnt), Marco Fechtenburg C… (Havre AC, DEN), Andrew Connor-Piper (FC Energie Cottbus, USA, Inj),
Stephen Buckingham (VfB Stuttgart, ISL), Valerio Anselmi (Casciavit, ITA, Wnt), Andrea Maffei (SSC Bari, ITA), Sumaila Djú (Paris FC, GNB),
Josué Lusamba (Rennes, FRA, Trn), Miljan Živanović (Crvena zvezda, SRB), Goran Stanojlović (Crvena zvezda, SRB), Alessio Kottelat (Wil, SUI),
Antonini Šemiga (Burnley, CRO), Maurice Biaye (Málaga, SEN), Vittorio Strano (Auxerre, FRA), Nguyen Viet Trieu (Valenciennes, FRA, Wnt),
Jan Stáňa (Sparta Prague, CZE, Wnt), Simon Skov (Havre AC, DEN), Gavin O'Neill (Falkirk, SCO, Trn), Oscar Maxsø (Lyngby, DEN, Wnt),
John Ramström (Cesena FC, SWE, Lst), Pablo Cubelo (Sevilla, ESP), Maurice Mougin (FC Den Bosch, FRA, Inj), Mamady Togola (Valencia B, MLI),
Aziz El Hariri (Villarreal, MAR; shows "Int" in the filtered view).

- **Pass Transfer only · Persuadable (18)**: Boutaleb, Kaplan, Forsberg, Mauricio Jiménez, Zbiciak, Anselmi, Djú, Živanović, Stanojlović, Strano,
  Nguyen Viet Trieu, Stáňa, Skov, Maxsø, Ramström, Mougin, Togola, El Hariri.
- **Fail (13)**: Keïta, Bayal, Vacher, Fechtenburg C…, Connor-Piper, Buckingham, Maffei, Lusamba (Trn), Kottelat, Šemiga, Biaye, O'Neill (Trn), Cubelo.

## Set 1 addendum — DR, Transfer only · Doubtful (in game, 2036-01-16)

13 players: Wong, Musongi, Sall, Øverenget, Cissokho, Cipriano, Nešić, Masson, Diallo, Scotti, Catherine, Garrido, Korshun.
Same as Transfer only · Persuadable and Interested. Lafleur and Tomašević are not listed, so they pass Transfer or loan · Doubtful only through the loan rule.
