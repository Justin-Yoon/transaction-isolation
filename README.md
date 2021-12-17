# Transaction Isolation Levels
Hi there, this is a small repo demonstrating different transaction anomalies and how they can be mitigated via transaction isolation levels.

### Codebase
The codebase is pretty small and should be self explanatory. It consists of two files [transaction_isolation.go](https://github.com/justin-yoon/transaction-isolation/blob/master/transaction_isolation.go) and [transaction_isolation_test.go](https://github.com/justin-yoon/transaction-isolation/blob/master/transaction_isolation_test.go).

`transaction_isolation.go` has a set of functions that each produce a sql anomaly, the test file runs these anomaly functions with different isolation levels.

To run the tests yourself, you will need to first set up the database using `docker-compose up -d` then run `go test ./...`
### Super Quick history
* [ANSI SQL](http://www.adp-gmbh.ch/ora/misc/isolation_level.html) first introduces anomalies dirty read, non-repeatable read and phantom read. Asserts that if we remove these anomalies we get serialization
* [Microsoft critique of ANSI SQL Isolation Levels](https://www.cs.umb.edu/cs734/CritiqueANSI_Iso.pdf) points out that eliminating the above 3 phenomena leads to Snapshot Isolation NOT Serialization.
* [Serializable Snapshot Isolation in PostgreSQL](https://arxiv.org/pdf/1208.4179.pdf) postgres implements SSI in 2012

