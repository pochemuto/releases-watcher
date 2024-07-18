-- public.actual_album определение

-- Drop table

-- DROP TABLE public.actual_album;

CREATE TABLE public.actual_album (
	id int8 NOT NULL,
	artist varchar COLLATE "ru-RU-x-icu" NULL,
	"name" varchar COLLATE "ru-RU-x-icu" NULL,
	"year" int4 NULL,
	kind varchar NULL,
	CONSTRAINT actual_album_pk PRIMARY KEY (id)
);


-- public.album определение

-- Drop table

-- DROP TABLE public.album;

CREATE TABLE public.album (
	artist varchar COLLATE "ru-RU-x-icu" NOT NULL,
	"name" varchar COLLATE "ru-RU-x-icu" NOT NULL,
	CONSTRAINT album_pk PRIMARY KEY (artist, name)
);


-- public."cache" определение

-- Drop table

-- DROP TABLE public."cache";

CREATE TABLE public."cache" (
	entity varchar NOT NULL,
	id varchar NOT NULL,
	value jsonb NULL,
	ts timestamp DEFAULT CURRENT_TIMESTAMP NULL,
	CONSTRAINT cache_pk PRIMARY KEY (entity, id)
);


CREATE TABLE public."excluded_artist" (
	"artist" varchar NOT NULL,
	PRIMARY KEY ("artist")
);