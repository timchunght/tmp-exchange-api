# Iosis Digital Asset Exchange.

## Dependencies

* mysql:5.6

```
(**BINLOG[ROW format]** enabled)
```

```
docker run --name mysql -p 3306:3306 -e MYSQL_ROOT_PASSWORD=password -d mysql:5.6 --server-id=1 --log-bin=mysql-bin --binlog-format=ROW
```
* kafka

```
docker-compose -f docker-compose.kafka.yml up -d
```

* redis

## Install

### Exchange API
* Create database and make sure **BINLOG[ROW format]** enabled
* Execute `ddl.sql`

```
 mysql -h 127.0.0.1 --port=3306 --database=iosis_ex_dev --password=password --user=root < ddl.sql
```

* Modify `conf.json`
* Run `go build`
* Run `./iosis-exchange-api`

### Exchange UI
* Run `yarn install`
* Run `yarn start`
* Run `yarn run build` to build production


