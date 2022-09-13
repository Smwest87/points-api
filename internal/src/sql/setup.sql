CREATE SCHEMA points_api;

CREATE TABLE points_api.point_transactions (
                                               ID  SERIAL primary key,
                                               payer text not null,
                                               points int not null,
                                               remainder int not null,
                                               created_at timestamp with time zone not null,
                                               updated_at timestamp with time zone

);