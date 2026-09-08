package com.example.refdata;

import java.util.UUID;

import org.apache.thrift.protocol.TBinaryProtocol;
import org.apache.thrift.transport.TSocket;
import org.apache.thrift.transport.TTransport;

import com.example.refdata.query.AssetClass;
import com.example.refdata.query.Instrument;
import com.example.refdata.query.InvalidQuery;
import com.example.refdata.query.LookupError;
import com.example.refdata.query.InstrumentQuery;
import com.example.refdata.query.InstrumentSelector;
import com.example.refdata.query.ReferenceDataQuery;

/** Java walkthrough client. Layer stack mirrors the server exactly. */
public class JavaClient {

  public static void main(String[] args) throws Exception {
    TTransport transport = new TSocket("127.0.0.1", 9090, 5000); // 5 s I/O timeout
    transport.open();
    try {
      ReferenceDataQuery.Client client =
          new ReferenceDataQuery.Client(new TBinaryProtocol(transport));

      InstrumentSelector bySymbol = new InstrumentSelector().setSymbol("AAPL");
      Instrument apple = client.getInstrument(bySymbol);
      System.out.println("getInstrument: " + apple.getSymbol() + " " + apple.getName());

      InstrumentQuery query = new InstrumentQuery()
          .setAssetClass(AssetClass.EQUITY)
          .setQuoteCurrency("EUR")
          .setLimit(10);
      System.out.println("findInstruments: " + client.findInstruments(query).size() + " match(es)");

      var snap = client.latestSnapshot(UUID.fromString("00000000-0000-4000-8000-000000000002"));
      System.out.println("latestSnapshot last: " + snap.getLast());

      System.out.println("exchangeDirectory XNAS: " + client.exchangeDirectory().get("XNAS"));

      client.reportUsage("java-walkthrough"); // oneway: returns immediately

      try {
        client.getInstrument(new InstrumentSelector().setSymbol("NOPE"));
      } catch (LookupError e) {
        System.out.println("LookupError as expected: " + e.getReason());
      }
      try {
        client.getInstrument(new InstrumentSelector());
      } catch (InvalidQuery e) {
        System.out.println("InvalidQuery as expected: " + e.getReason());
      }
    } finally {
      transport.close();
    }
  }
}
