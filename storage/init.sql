create table if not exists url_list
(
    id bigint not null
    constraint shorten_id
    primary key,
    original_url varchar(255) not null,
    shorted_url varchar(255) not null,
    clicks int not null
);