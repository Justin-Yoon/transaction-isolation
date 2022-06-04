# Transaction Isolation Levels
Hi there, this repo has 2 important functions.

The first is a small amount of [Go code](#codebase) that shows different transaction anomalies and how they can be mitigated via transaction isolation levels.

The second is the rest of this README, a brain dump of sorts of everything I've learned about transaction isolation levels as I've slowly jumped into this rabbit hole.

### Codebase
The codebase is pretty small and should be self explanatory. It consists of two files [transaction_isolation.go](https://github.com/justin-yoon/transaction-isolation/blob/master/transaction_isolation.go) and [transaction_isolation_test.go](https://github.com/justin-yoon/transaction-isolation/blob/master/transaction_isolation_test.go).

`transaction_isolation.go` has a set of functions that each produce a sql anomaly, the test file runs these anomaly functions with different isolation levels.

To run the tests yourself, you will need to first set up the database using `docker-compose up -d` then run `go test ./...`
### Super Quick history
* [ANSI SQL](http://www.adp-gmbh.ch/ora/misc/isolation_level.html) first introduces anomalies dirty read, non-repeatable read and phantom read. Asserts that if we remove these anomalies we get serialization
* [Microsoft critique of ANSI SQL Isolation Levels](https://www.cs.umb.edu/cs734/CritiqueANSI_Iso.pdf) points out that eliminating the above 3 phenomena leads to Snapshot Isolation NOT Serialization.
* [Serializable Snapshot Isolation in PostgreSQL](https://arxiv.org/pdf/1208.4179.pdf) postgres implements SSI in 2012

### Snapshot Isolation
MVCC holds tuple (row) level locks. Checks for conflicts and rollsback

### Serializable Snapshot Isolation
### Serializability
When 2 transactions are run concurrently and if there is some dependency amonst them, the database needs to decide which transaction to commit first to guarantee serializability.

By far the most important type of dependency for this check is a read write dependency or `rw-antidependency`

Suppose there is a T1 that reads a row and a T2 that concurrently writes to that same row. T1 must come before T2 because T1's read is overwritten by T2.

Theory

Note that this dangerous structure does not necessarily mean that a serialization anomaly will happen only that it is possible. Precisely Serializable Snapshot Isolation is an extension to SSI that can eliminate all false positives but is not implemented in Postgres due to performance reasons.

### Related Links
[History of transaction isolation levels](https://ristret.com/s/f643zk/history_transaction_histories)

# Related topics
Explicit locks
SELECT FOR UPDATE, SELECT FOR SHARE
SERIALIZABLE READ ONLY DEFERRABLE



TODO
* fix packages
* understand serialization more
* coments for write skew, lost update

