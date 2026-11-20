---
title: Deutsch setzen
author: setdown
date: 2000-01-01
lang: de
page-numbers: Seite 1 von 5
---

Mit `lang: de` in der Front Matter oder `--lang de` auf der Kommandozeile setzt setdown deutsch: mit deutscher Silbentrennung, mit „Anführungszeichen“ aus "geraden" und ‚halben‘ aus 'einfachen' Zeichen, mit Gedankenstrichen aus `--` -- wie hier -- und mit Auslassungspunkten aus drei Punkten...

## Silbentrennung

Deutsche Texte sind voller langer Komposita. Typst trennt sie nach deutschen Regeln und bricht den Absatz als Ganzes um, so bleiben die Wortabstände im Blocksatz gleichmäßig, auch bei Wörtern wie Donaudampfschifffahrtsgesellschaftskapitän, Rechtsschutzversicherungsgesellschaften oder Grundstücksverkehrsgenehmigungszuständigkeitsübertragungsverordnung.

## Anführungszeichen

Gerade Anführungszeichen werden zu deutschen: "doppelte" und 'einfache', auch "verschachtelt 'wie hier'". Direkt getippte Zeichen werden angepasst: “englische” Zeichen werden zu deutschen, „deutsche“ bleiben, wie sie sind.

## Paragrafen und Abkürzungen

Ein Paragrafenzeichen und die Abkürzungen Abs., Nr. und Art. bleiben auf der Zeile der Zahl, die folgt. So trennt der Umbruch nie § 13 Abs. 3 Nr. 1 BGB oder Art. 5 GG, und §§ 312 ff. bleiben zusammen, wo immer die Zeile endet. Dieser Satz ist lang genug, dass solche Angaben an verschiedenen Stellen der Zeile landen, etwa § 433 Abs. 1 BGB, Art. 20 Abs. 3 GG und § 1 Nr. 2.

## Listen und Tabellen

Ein Absatz, der mit einem Doppelpunkt endet, bleibt bei der Liste, die er einleitet:

1. Bis Ende 2026:
   - Schriften wählen
   - Vorlage schreiben
2. Bis zum Frühjahr 2027:
   - erste Version veröffentlichen

| Schrift        | Verwendung             | Schnitte |
|:---------------|:-----------------------|---------:|
| Source Serif 4 | Fließtext (Standard)   |       12 |
| Source Sans 3  | Fließtext mit `--sans` |       14 |
| JetBrains Mono | Code                   |       16 |

## Seitenzahlen

Die Seitenzahlen in der Fußzeile gibt `page-numbers: Seite 1 von 5` in der Front Matter vor: Die erste Zahl steht für die Seite, die zweite für die Anzahl der Seiten, alles andere bleibt, wie es ist.

## Schweizer Rechtschreibung

<!--raw-typst #set text(region: "CH")-->

Mit `lang: de-CH` setzt setdown die Anführungszeichen der Schweiz: "Guillemets" und 'halbe Guillemets'. Dieser Abschnitt schaltet die Region mit einem Typst-Befehl in einem HTML-Kommentar um, für alles, was folgt.
