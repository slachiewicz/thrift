# Python walkthrough — Reference Data Query (Apache Thrift 0.24.0)
#
# Server: TSimpleServer over a TServerSocket (blocking, single connection at
# a time), buffered transport layer, binary protocol. These are deliberate,
# conservative choices for a first runnable example; see the guide's
# "Runtime choices" chapter for alternatives.

import glob
import logging
import os
import sys
import uuid as uuidlib

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "gen-py"))

from refdata.common import constants as common_constants
from refdata.common.ttypes import MarketPhase, SnapshotContext
from refdata.query import ReferenceDataQuery
from refdata.query.ttypes import (
    AssetClass,
    Instrument,
    InstrumentQuery,
    InstrumentSelector,
    InvalidQuery,
    LookupError,
    QuoteSnapshot,
)

from thrift.protocol import TBinaryProtocol
from thrift.server import TServer
from thrift.transport import TSocket, TTransport

LOG = logging.getLogger("refdata.server")

NOW_MS = 1767000000000  # fixed as-of for reproducible output


def _instrument(iid, symbol, name, asset_class, mic, isin=None, currencies=None):
    return Instrument(
        instrumentId=iid,
        symbol=symbol,
        name=name,
        assetClass=asset_class,
        primaryMic=mic,
        isin=isin,
        quoteCurrencies=currencies,
        tags=None,
        attributes=None,
        context=SnapshotContext(
            asOf=NOW_MS,
            sourceId="walkthrough-static",
            phase=MarketPhase.OPEN,
            quoteCurrency="USD",
        ),
    )


def _uuid(s):
    return uuidlib.UUID(s)


INSTRUMENTS = {
    _uuid("00000000-0000-4000-8000-000000000001").bytes: _instrument(
        _uuid("00000000-0000-4000-8000-000000000001"),
        "AAPL", "Apple Inc.", AssetClass.EQUITY, "XNAS",
        isin="US0378331005", currencies=set(["USD"]),
    ),
    _uuid("00000000-0000-4000-8000-000000000002").bytes: _instrument(
        _uuid("00000000-0000-4000-8000-000000000002"),
        "MSFT", "Microsoft Corporation", AssetClass.EQUITY, "XNAS",
        isin="US5949181045", currencies=set(["USD"]),
    ),
    _uuid("00000000-0000-4000-8000-000000000003").bytes: _instrument(
        _uuid("00000000-0000-4000-8000-000000000003"),
        "DTE", "Deutsche Telekom AG", AssetClass.EQUITY, "XETR",
        isin="DE0005557508", currencies=set(["EUR"]),
    ),
}


class ReferenceDataHandler(object):
    """Implements refdata.query.ReferenceDataQuery.Iface."""

    def getInstrument(self, selector):
        # Union convention: exactly one member set. Generated code does not
        # enforce this uniformly across languages (Go 0.24.0 writes all
        # members, including zero values), so handlers validate leniently:
        # a member counts as set only if it is non-empty/non-zero.
        chosen = []
        if selector.instrumentId is not None and selector.instrumentId.int != 0:
            chosen.append("instrumentId")
        if selector.symbol:
            chosen.append("symbol")
        if selector.isin:
            chosen.append("isin")
        if len(chosen) != 1:
            raise InvalidQuery(
                reason="selector must set exactly one field (non-empty)",
                violations=["instrumentId|symbol|isin"],
            )
        if selector.symbol is not None:
            for instr in INSTRUMENTS.values():
                if instr.symbol == selector.symbol:
                    return instr
            raise LookupError(reason="unknown symbol: %s" % selector.symbol)
        key = selector.instrumentId.bytes
        if key not in INSTRUMENTS:
            raise LookupError(reason="unknown instrumentId", instrumentId=selector.instrumentId)
        return INSTRUMENTS[key]

    def findInstruments(self, query):
        limit = query.limit if query.limit else common_constants.DEFAULT_PAGE_SIZE
        if limit > common_constants.MAX_PAGE_SIZE:
            raise InvalidQuery(
                reason="limit exceeds maximum",
                violations=["limit>%d" % common_constants.MAX_PAGE_SIZE],
            )
        out = []
        for instr in INSTRUMENTS.values():
            if query.symbolPrefix and not instr.symbol.startswith(query.symbolPrefix):
                continue
            if query.assetClass is not None and instr.assetClass != query.assetClass:
                continue
            if query.mic is not None and instr.primaryMic != query.mic:
                continue
            if query.quoteCurrency is not None and (
                    not instr.quoteCurrencies or query.quoteCurrency not in instr.quoteCurrencies):
                continue
            out.append(instr)
            if len(out) >= limit:
                break
        return out

    def latestSnapshot(self, instrumentId):
        instr = INSTRUMENTS.get(instrumentId.bytes)
        if instr is None:
            raise LookupError(reason="unknown instrumentId", instrumentId=instrumentId)
        # Deterministic pseudo-prices keep the walkthrough reproducible.
        seed = int(instrumentId.bytes[-1])
        return QuoteSnapshot(
            instrumentId=instrumentId,
            asOf=NOW_MS,
            bid=100.0 + seed - 0.25,
            ask=100.0 + seed + 0.25,
            last=100.0 + seed,
            volume=10000 + 100 * seed,
            phase=MarketPhase.OPEN,
        )

    def exchangeDirectory(self):
        return {
            "XNAS": "NASDAQ Stock Market",
            "XETR": "Deutsche Boerse Xetra",
            "XNYS": "New York Stock Exchange",
        }

    def reportUsage(self, clientApplication):
        # oneway: returns nothing, cannot throw back, may run after the reply.
        LOG.info("usage report from %s", clientApplication)


def main():
    logging.basicConfig(level=logging.INFO, format="%(levelname)s %(message)s")
    handler = ReferenceDataHandler()
    processor = ReferenceDataQuery.Processor(handler)
    # Blocking accept loop; one request at a time (simple, safe, slow).
    server_transport = TSocket.TServerSocket(host="127.0.0.1", port=9090)
    transport_factory = TTransport.TBufferedTransportFactory()
    protocol_factory = TBinaryProtocol.TBinaryProtocolFactory()
    server = TServer.TSimpleServer(processor, server_transport, transport_factory, protocol_factory)
    LOG.info("ReferenceDataQuery listening on 127.0.0.1:9090")
    server.serve()
    return 0


if __name__ == "__main__":
    sys.exit(main())
