-- The interview question, owned by the room rather than hardcoded in the
-- client. Interviewers change it per interview; candidates only read it.
--
-- NOT NULL DEFAULT '' rather than nullable: an absent question and an empty
-- one mean the same thing to every reader, and a nullable column would make
-- every one of them handle a case that carries no extra information.
ALTER TABLE rooms
    ADD COLUMN prompt TEXT NOT NULL DEFAULT '';
