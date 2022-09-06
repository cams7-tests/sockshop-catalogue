FROM postgres:14.5

ENV POSTGRES_USER=postgres
ENV POSTGRES_PASSWORD=postgrespass
ENV CATALOGUE_DB_USER=catalogue_user
ENV CATALOGUE_DB_PASS=default_password
ENV CATALOGUE_DB_NAME=socksdb

COPY ./data/dump.sql /tmp/
COPY ./data/create-db.sh /docker-entrypoint-initdb.d/

