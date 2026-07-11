# Dataset provenance

One JSON file per BS year, hand-curated: national public holidays of
Nepal with English and Nepali names. Only the BS date is stored — the
AD equivalent is computed at runtime by `internal/calendar`, so the two
can never disagree.

Curation process (see `cmd/curate-holidays`):

1. Candidate days extracted from the holiday flags in
   [S4NKALP/nepali-calendar-api](https://github.com/S4NKALP/nepali-calendar-api)
   (MIT), a scrape of the hamropatro panchang.
2. Weekly Saturday holidays dropped; every remaining entry verified and
   named by hand against public holiday announcements.
3. Known flag errors corrected manually (in the scrape, Holi and
   Prithvi Jayanti carry no holiday flag; both are national holidays).

Deliberate exclusions:

- **Islamic holidays (Eid al-Fitr, Eid al-Adha, Bakr Id)** — dates are
  set by moon sighting and announced close to the day; they are added
  per-year once confirmed rather than guessed.
- Valley- or community-scoped observances (Indra Jatra, Gai Jatra) and
  employee-group holidays (Teej for women employees) — the dataset
  covers nationwide public holidays; the two-day Holi split
  (hilly/Terai districts) is included with a `note` because together
  the days cover the whole country.

Nepal's holiday list is announced yearly by the government and can
change (holidays are added and removed by cabinet decision); each
year's file reflects the list as published for that year.
