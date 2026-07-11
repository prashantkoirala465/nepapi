# Dataset attribution

`days_in_month.json` (BS month lengths, 1970–2090 BS) is derived from
[medic/bikram-sambat](https://github.com/medic/bikram-sambat)
(`test-data/daysInMonth.json`), licensed under the
[Apache License 2.0](https://github.com/medic/bikram-sambat/blob/master/LICENSE),
and is used in production healthcare software in Nepal.

`../testdata/toGreg.json` (30,572 verified BS→AD conversion pairs used
by the test suite) comes from the same project.

Month lengths in the Bikram Sambat calendar are set by astronomical
calculation and published per year; there is no closed-form rule, so
every converter is ultimately table-driven from data like this.
