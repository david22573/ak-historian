# PR4B0-R1P4 collection protocol

This is the frozen pre-acquisition authority for the open prospective generation `ak-historian-binance-futures-um-1m-prospective-pit-r1`.

It permits only unauthenticated public Binance USD-M futures `1m` server-time and kline GET requests for the exact nine-symbol universe. The collector runs every five minutes, retains only provider-time-complete candles, records actual later availability for catch-up data, and maintains append-only accepted-receipt and acquisition-envelope hash chains.

The paired JSON is normative. Candidate evaluation, Engine execution, outcome inspection, and real RIF lifecycle state are prohibited.
