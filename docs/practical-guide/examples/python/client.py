# Python walkthrough — Reference Data Query client (Apache Thrift 0.24.0)

import os
import sys
import uuid as uuidlib

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "gen-py"))

from refdata.query import ReferenceDataQuery
from refdata.query.ttypes import (
    AssetClass,
    InstrumentQuery,
    InstrumentSelector,
    InvalidQuery,
    LookupError,
)

from thrift.protocol import TBinaryProtocol
from thrift.transport import TSocket, TTransport


def main():
    # Layer stack must MIRROR the server: socket -> buffered -> binary.
    sock = TSocket.TSocket(host="127.0.0.1", port=9090)
    sock.setTimeout(5000)  # ms; applies to connect and I/O on this handle
    transport = TTransport.TBufferedTransport(sock)
    protocol = TBinaryProtocol.TBinaryProtocol(transport)
    client = ReferenceDataQuery.Client(protocol)

    transport.open()
    try:
        apple = client.getInstrument(InstrumentSelector(symbol="AAPL"))
        print("getInstrument:", apple.symbol, apple.name, apple.primaryMic)

        equities = client.findInstruments(
            InstrumentQuery(assetClass=AssetClass.EQUITY, quoteCurrency="EUR", limit=10))
        print("findInstruments:", sorted(i.symbol for i in equities))

        snap = client.latestSnapshot(
            uuidlib.UUID("00000000-0000-4000-8000-000000000002"))
        print("latestSnapshot:", snap.last, "asOf", snap.asOf)

        print("exchangeDirectory:", client.exchangeDirectory()["XNAS"])

        client.reportUsage("python-walkthrough")  # oneway: returns immediately

        # Typed exception on the happy-path contract:
        try:
            client.getInstrument(InstrumentSelector(symbol="NOPE"))
        except LookupError as exc:
            print("LookupError as expected:", exc.reason)

        # Handler-side validation surfaced as a typed exception:
        try:
            client.getInstrument(InstrumentSelector())
        except InvalidQuery as exc:
            print("InvalidQuery as expected:", exc.reason)
    finally:
        transport.close()
    return 0


if __name__ == "__main__":
    sys.exit(main())
