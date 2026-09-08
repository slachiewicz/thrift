package com.example.refdata;

import java.util.HashMap;
import java.util.Map;
import java.util.UUID;

import org.apache.thrift.TException;
import org.apache.thrift.protocol.TBinaryProtocol;
import org.apache.thrift.server.TServer;
import org.apache.thrift.server.TSimpleServer;
import org.apache.thrift.transport.TServerSocket;
import org.apache.thrift.transport.TTransportException;
import org.apache.thrift.transport.TTransportFactory;

import com.example.refdata.common.MarketPhase;
import com.example.refdata.common.SnapshotContext;
import com.example.refdata.query.InvalidQuery;
import com.example.refdata.query.LookupError;
import com.example.refdata.query.ReferenceDataQuery;
import com.example.refdata.query.Instrument;
import com.example.refdata.query.InstrumentQuery;
import com.example.refdata.query.InstrumentSelector;
import com.example.refdata.query.QuoteSnapshot;

/**
 * Java walkthrough server. Layer stack (must be mirrored by clients):
 * TServerSocket -> buffered transport -> binary protocol, TSimpleServer.
 */
public class JavaServer {

  static final long NOW_MS = 1767000000000L; // fixed as-of, reproducible output

  static UUID uid(String s) {
    return UUID.fromString(s);
  }

  static class Handler implements ReferenceDataQuery.Iface {

    private final Map<UUID, Instrument> instruments = new HashMap<>();

    Handler() {
      instruments.put(uid("00000000-0000-4000-8000-000000000001"),
          new Instrument(uid("00000000-0000-4000-8000-000000000001"), "AAPL",
              "Apple Inc.", com.example.refdata.query.AssetClass.EQUITY, "XNAS")
              .setIsin("US0378331005")
              .setQuoteCurrencies(java.util.Set.of("USD"))
              .setContext(new SnapshotContext(NOW_MS)
                  .setSourceId("walkthrough-static")
                  .setPhase(MarketPhase.OPEN)
                  .setQuoteCurrency("USD")));
      instruments.put(uid("00000000-0000-4000-8000-000000000002"),
          new Instrument(uid("00000000-0000-4000-8000-000000000002"), "MSFT",
              "Microsoft Corporation", com.example.refdata.query.AssetClass.EQUITY, "XNAS")
              .setIsin("US5949181045")
              .setQuoteCurrencies(java.util.Set.of("USD")));
      instruments.put(uid("00000000-0000-4000-8000-000000000003"),
          new Instrument(uid("00000000-0000-4000-8000-000000000003"), "DTE",
              "Deutsche Telekom AG", com.example.refdata.query.AssetClass.EQUITY, "XETR")
              .setIsin("DE0005557508")
              .setQuoteCurrencies(java.util.Set.of("EUR")));
    }

    @Override
    public Instrument getInstrument(InstrumentSelector selector) throws LookupError, InvalidQuery {
      // Union convention: exactly one member set. Generated Java code does
      // not enforce it, so the handler validates.
      int set = 0;
      if (selector.getInstrumentId() != null) set++;
      if (selector.getSymbol() != null && !selector.getSymbol().isEmpty()) set++;
      if (selector.getIsin() != null && !selector.getIsin().isEmpty()) set++;
      if (set != 1) {
        throw new InvalidQuery("selector must set exactly one field");
      }
      if (selector.getSymbol() != null) {
        for (Instrument i : instruments.values()) {
          if (i.getSymbol().equals(selector.getSymbol())) {
            return i;
          }
        }
        throw new LookupError("unknown symbol: " + selector.getSymbol());
      }
      Instrument found = instruments.get(selector.getInstrumentId());
      if (found == null) {
        throw new LookupError("unknown instrumentId").setInstrumentId(selector.getInstrumentId());
      }
      return found;
    }

    @Override
    public java.util.List<Instrument> findInstruments(InstrumentQuery query) throws InvalidQuery {
      int limit = query.isSetLimit() ? query.getLimit() : 20;
      if (limit > 500) {
        throw new InvalidQuery("limit exceeds maximum").setViolations(java.util.List.of("limit>500"));
      }
      java.util.List<Instrument> out = new java.util.ArrayList<>();
      for (Instrument i : instruments.values()) {
        if (query.isSetSymbolPrefix() && !i.getSymbol().startsWith(query.getSymbolPrefix())) continue;
        if (query.isSetAssetClass() && i.getAssetClass() != query.getAssetClass()) continue;
        if (query.isSetMic() && !i.getPrimaryMic().equals(query.getMic())) continue;
        if (query.isSetQuoteCurrency()
            && (i.getQuoteCurrencies() == null || !i.getQuoteCurrencies().contains(query.getQuoteCurrency()))) {
          continue;
        }
        out.add(i);
        if (out.size() >= limit) break;
      }
      return out;
    }

    @Override
    public QuoteSnapshot latestSnapshot(UUID instrumentId) throws LookupError {
      if (!instruments.containsKey(instrumentId)) {
        throw new LookupError("unknown instrumentId").setInstrumentId(instrumentId);
      }
      int seed = (int) instrumentId.getLeastSignificantBits();
      return new QuoteSnapshot(instrumentId, NOW_MS)
          .setBid(100.0 + seed - 0.25)
          .setAsk(100.0 + seed + 0.25)
          .setLast(100.0 + seed)
          .setVolume(10000L + 100L * seed)
          .setPhase(MarketPhase.OPEN);
    }

    @Override
    public Map<String, String> exchangeDirectory() {
      Map<String, String> dir = new HashMap<>();
      dir.put("XNAS", "NASDAQ Stock Market");
      dir.put("XETR", "Deutsche Boerse Xetra");
      dir.put("XNYS", "New York Stock Exchange");
      return dir;
    }

    @Override
    public void reportUsage(String clientApplication) {
      // oneway: no reply, no error path back to the caller.
      System.out.println("usage report from " + clientApplication);
    }
  }

  public static void main(String[] args) throws TTransportException {
    ReferenceDataQuery.Processor processor = new ReferenceDataQuery.Processor(new Handler());
    TServerSocket serverSocket = new TServerSocket(9090);
    TServer server = new TSimpleServer(new TServer.Args(serverSocket)
        .processor(processor)
        .transportFactory(new TTransportFactory())
        .protocolFactory(new TBinaryProtocol.Factory()));
    System.out.println("ReferenceDataQuery listening on 127.0.0.1:9090");
    server.serve();
  }
}
