---
title: Invoice 2026-0815
author: Typesetting Supplies GmbH, Berlin
date: November 21, 2026
---

Example Print Shop AG\
Beispielweg 2\
80331 München, Germany

Your reference: 04011000-12345-34 · Delivered on November 20, 2026

| Item                           | Quantity | Unit price |  Amount |
|:-------------------------------|---------:|-----------:|--------:|
| Composing stick, 12 inch       |        1 |     €48.00 |  €48.00 |
| Brass rule, 2 point, set of 10 |        2 |     €36.50 |  €73.00 |
| Shipping                       |        1 |      €9.90 |   €9.90 |
| Net amount                     |          |            | €130.90 |
| VAT 19 %                       |          |            |  €24.87 |
| **Total**                      |          |            | **€155.77** |

Payable within 30 days without deduction, by December 21, 2026, to IBAN DE89 3704 0044 0532 0130 00, quoting the invoice number. VAT ID DE123456789. Questions: Jane Doe, +49 30 1234567, jane.doe@example.com.

## How this PDF is made

This invoice is a Factur-X and ZUGFeRD e-invoice: a PDF/A-3b whose pages people read, with the same invoice attached as XML for accounting software. Its `options.txt` holds the option it is rendered with:

```bash
setdown --invoice factur-x.xml pdf-a3b.md
```

setdown reads the profile from the XML (here EN 16931), makes the PDF at `--pdf-level a3b`, attaches the XML as `factur-x.xml` with the relationship `alternative`, and adds the metadata the two formats require to the PDF's XMP: the document type, the XML's file name, the version and the profile. Mustang, the open source validator for ZUGFeRD and Factur-X, checks such files in every profile in setdown's CI.

The profiles BASIC, EN 16931, EXTENDED and XRECHNUNG work; the XML for XRECHNUNG is attached as `xrechnung.xml`. MINIMUM and BASIC WL carry no full invoice, so setdown refuses them.

## What stays the sender's job

- **The XML** comes from the accounting software or an e-invoicing library; setdown does not write it. The one here is a sample.
- **The pages and the XML must agree.**
- **Sending:** where a country requires a platform or network for e-invoices, the file has to go through it.
