--
-- PostgreSQL database dump
--

-- Dumped from database version 17.5
-- Dumped by pg_dump version 17.5

SET statement_timeout = 0;
SET lock_timeout = 0;
SET idle_in_transaction_session_timeout = 0;
SET transaction_timeout = 0;
SET client_encoding = 'UTF8';
SET standard_conforming_strings = on;
SELECT pg_catalog.set_config('search_path', '', false);
SET check_function_bodies = false;
SET xmloption = content;
SET client_min_messages = warning;
SET row_security = off;

--
-- Name: citext; Type: EXTENSION; Schema: -; Owner: -
--

CREATE EXTENSION IF NOT EXISTS citext WITH SCHEMA public;


--
-- Name: EXTENSION citext; Type: COMMENT; Schema: -; Owner: 
--

COMMENT ON EXTENSION citext IS 'data type for case-insensitive character strings';


--
-- Name: uuid-ossp; Type: EXTENSION; Schema: -; Owner: -
--

CREATE EXTENSION IF NOT EXISTS "uuid-ossp" WITH SCHEMA public;


--
-- Name: EXTENSION "uuid-ossp"; Type: COMMENT; Schema: -; Owner: 
--

COMMENT ON EXTENSION "uuid-ossp" IS 'generate universally unique identifiers (UUIDs)';


--
-- Name: partner_actor_type_enum; Type: TYPE; Schema: public; Owner: postgres
--

CREATE TYPE public.partner_actor_type_enum AS ENUM (
    'system',
    'partner_user',
    'partner'
);


ALTER TYPE public.partner_actor_type_enum OWNER TO postgres;

--
-- Name: partner_status_enum; Type: TYPE; Schema: public; Owner: postgres
--

CREATE TYPE public.partner_status_enum AS ENUM (
    'active',
    'suspended'
);


ALTER TYPE public.partner_status_enum OWNER TO postgres;

--
-- Name: role_enum; Type: TYPE; Schema: public; Owner: postgres
--

CREATE TYPE public.role_enum AS ENUM (
    'system_admin',
    'partner_admin',
    'partner_user',
    'trader'
);


ALTER TYPE public.role_enum OWNER TO postgres;

--
-- Name: user_account_status; Type: TYPE; Schema: public; Owner: postgres
--

CREATE TYPE public.user_account_status AS ENUM (
    'active',
    'suspended',
    'restricted',
    'pending_registration'
);


ALTER TYPE public.user_account_status OWNER TO postgres;

--
-- Name: set_updated_at(); Type: FUNCTION; Schema: public; Owner: postgres
--

CREATE FUNCTION public.set_updated_at() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
   NEW.updated_at = NOW();
   RETURN NEW;
END;
$$;


ALTER FUNCTION public.set_updated_at() OWNER TO postgres;

SET default_tablespace = '';

SET default_table_access_method = heap;

--
-- Name: account_deletion_requests; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.account_deletion_requests (
    id bigint NOT NULL,
    user_id bigint,
    reason text,
    requested_at timestamp with time zone DEFAULT now(),
    processed_at timestamp with time zone,
    processed_by bigint
);


ALTER TABLE public.account_deletion_requests OWNER TO postgres;

--
-- Name: countries; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.countries (
    id integer NOT NULL,
    iso2 character(2) NOT NULL,
    iso3 character(3) NOT NULL,
    name text NOT NULL,
    phone_code text,
    currency_code character(3),
    currency_name text,
    region text,
    subregion text,
    flag_url text,
    created_at timestamp with time zone DEFAULT now(),
    updated_at timestamp with time zone DEFAULT now()
);


ALTER TABLE public.countries OWNER TO postgres;

--
-- Name: countries_id_seq; Type: SEQUENCE; Schema: public; Owner: postgres
--

CREATE SEQUENCE public.countries_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER SEQUENCE public.countries_id_seq OWNER TO postgres;

--
-- Name: countries_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: postgres
--

ALTER SEQUENCE public.countries_id_seq OWNED BY public.countries.id;


--
-- Name: email_logs; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.email_logs (
    id bigint NOT NULL,
    user_id text,
    subject text,
    recipient_email text,
    type text,
    status text DEFAULT 'sent'::text,
    sent_at timestamp with time zone DEFAULT now()
);


ALTER TABLE public.email_logs OWNER TO postgres;

--
-- Name: kyc_audit_logs; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.kyc_audit_logs (
    id bigint NOT NULL,
    kyc_id bigint NOT NULL,
    action character varying(50) NOT NULL,
    actor character varying(100),
    notes text,
    created_at timestamp without time zone DEFAULT now() NOT NULL
);


ALTER TABLE public.kyc_audit_logs OWNER TO postgres;

--
-- Name: kyc_audit_logs_id_seq; Type: SEQUENCE; Schema: public; Owner: postgres
--

CREATE SEQUENCE public.kyc_audit_logs_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER SEQUENCE public.kyc_audit_logs_id_seq OWNER TO postgres;

--
-- Name: kyc_audit_logs_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: postgres
--

ALTER SEQUENCE public.kyc_audit_logs_id_seq OWNED BY public.kyc_audit_logs.id;


--
-- Name: kyc_submissions; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.kyc_submissions (
    id bigint NOT NULL,
    user_id bigint NOT NULL,
    id_number character varying(100) NOT NULL,
    nationality character varying(100),
    document_type character varying(50) DEFAULT 'national_id'::character varying NOT NULL,
    document_front_url text NOT NULL,
    document_back_url text NOT NULL,
    status character varying(30) DEFAULT 'pending'::character varying NOT NULL,
    rejection_reason text,
    submitted_at timestamp without time zone DEFAULT now() NOT NULL,
    reviewed_at timestamp without time zone,
    updated_at timestamp without time zone DEFAULT now() NOT NULL,
    selfie_image_url text,
    date_of_birth date,
    consent boolean DEFAULT true NOT NULL
);


ALTER TABLE public.kyc_submissions OWNER TO postgres;

--
-- Name: kyc_submissions_id_seq; Type: SEQUENCE; Schema: public; Owner: postgres
--

CREATE SEQUENCE public.kyc_submissions_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER SEQUENCE public.kyc_submissions_id_seq OWNER TO postgres;

--
-- Name: kyc_submissions_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: postgres
--

ALTER SEQUENCE public.kyc_submissions_id_seq OWNED BY public.kyc_submissions.id;


--
-- Name: oauth_accounts; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.oauth_accounts (
    id bigint NOT NULL,
    user_id bigint,
    provider text NOT NULL,
    provider_uid text NOT NULL,
    access_token text,
    refresh_token text,
    linked_at timestamp with time zone DEFAULT now()
);


ALTER TABLE public.oauth_accounts OWNER TO postgres;

--
-- Name: partner_audit_logs; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.partner_audit_logs (
    id bigint NOT NULL,
    actor_type public.partner_actor_type_enum NOT NULL,
    actor_id bigint,
    action text NOT NULL,
    target_type text,
    target_id text,
    metadata jsonb,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


ALTER TABLE public.partner_audit_logs OWNER TO postgres;

--
-- Name: partner_audit_logs_id_seq; Type: SEQUENCE; Schema: public; Owner: postgres
--

CREATE SEQUENCE public.partner_audit_logs_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER SEQUENCE public.partner_audit_logs_id_seq OWNER TO postgres;

--
-- Name: partner_audit_logs_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: postgres
--

ALTER SEQUENCE public.partner_audit_logs_id_seq OWNED BY public.partner_audit_logs.id;


--
-- Name: partner_configs; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.partner_configs (
    partner_id text NOT NULL,
    default_fx_spread numeric(8,6) DEFAULT 0.005,
    webhook_secret text,
    config_data jsonb,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


ALTER TABLE public.partner_configs OWNER TO postgres;

--
-- Name: partner_kyc; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.partner_kyc (
    partner_id text NOT NULL,
    status text DEFAULT 'pending'::text NOT NULL,
    kyc_data jsonb,
    limits jsonb,
    risk_flags jsonb,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


ALTER TABLE public.partner_kyc OWNER TO postgres;

--
-- Name: partner_users; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.partner_users (
    id text NOT NULL,
    partner_id text NOT NULL,
    role character varying(16) NOT NULL,
    user_id bigint NOT NULL,
    is_active boolean DEFAULT true NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT partner_users_role_check CHECK (((role)::text = ANY ((ARRAY['partner_admin'::character varying, 'partner_user'::character varying])::text[])))
);


ALTER TABLE public.partner_users OWNER TO postgres;

--
-- Name: partner_users_id_seq; Type: SEQUENCE; Schema: public; Owner: postgres
--

CREATE SEQUENCE public.partner_users_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER SEQUENCE public.partner_users_id_seq OWNER TO postgres;

--
-- Name: partner_users_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: postgres
--

ALTER SEQUENCE public.partner_users_id_seq OWNED BY public.partner_users.id;


--
-- Name: partners; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.partners (
    id text NOT NULL,
    name text NOT NULL,
    country text,
    contact_email text,
    contact_phone text,
    status public.partner_status_enum DEFAULT 'active'::public.partner_status_enum NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);


ALTER TABLE public.partners OWNER TO postgres;

--
-- Name: rbac_modules; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.rbac_modules (
    id bigint NOT NULL,
    parent_id bigint,
    code text NOT NULL,
    name text,
    meta jsonb,
    is_active boolean DEFAULT true,
    created_at timestamp with time zone DEFAULT now(),
    created_by bigint,
    updated_at timestamp with time zone,
    updated_by bigint
);


ALTER TABLE public.rbac_modules OWNER TO postgres;

--
-- Name: rbac_modules_id_seq; Type: SEQUENCE; Schema: public; Owner: postgres
--

CREATE SEQUENCE public.rbac_modules_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER SEQUENCE public.rbac_modules_id_seq OWNER TO postgres;

--
-- Name: rbac_modules_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: postgres
--

ALTER SEQUENCE public.rbac_modules_id_seq OWNED BY public.rbac_modules.id;


--
-- Name: rbac_permission_types; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.rbac_permission_types (
    id bigint NOT NULL,
    code text NOT NULL,
    description text,
    is_active boolean DEFAULT true,
    created_at timestamp with time zone DEFAULT now(),
    created_by bigint,
    updated_at timestamp with time zone,
    updated_by bigint
);


ALTER TABLE public.rbac_permission_types OWNER TO postgres;

--
-- Name: rbac_permission_types_id_seq; Type: SEQUENCE; Schema: public; Owner: postgres
--

CREATE SEQUENCE public.rbac_permission_types_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER SEQUENCE public.rbac_permission_types_id_seq OWNER TO postgres;

--
-- Name: rbac_permission_types_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: postgres
--

ALTER SEQUENCE public.rbac_permission_types_id_seq OWNED BY public.rbac_permission_types.id;


--
-- Name: rbac_permissions_audit; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.rbac_permissions_audit (
    id bigint NOT NULL,
    actor_id bigint,
    object_type text,
    object_id bigint,
    action text,
    payload jsonb,
    created_at timestamp with time zone DEFAULT now()
);


ALTER TABLE public.rbac_permissions_audit OWNER TO postgres;

--
-- Name: rbac_permissions_audit_id_seq; Type: SEQUENCE; Schema: public; Owner: postgres
--

CREATE SEQUENCE public.rbac_permissions_audit_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER SEQUENCE public.rbac_permissions_audit_id_seq OWNER TO postgres;

--
-- Name: rbac_permissions_audit_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: postgres
--

ALTER SEQUENCE public.rbac_permissions_audit_id_seq OWNED BY public.rbac_permissions_audit.id;


--
-- Name: rbac_role_permissions; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.rbac_role_permissions (
    id bigint NOT NULL,
    role_id bigint NOT NULL,
    module_id bigint NOT NULL,
    submodule_id bigint,
    permission_type_id bigint NOT NULL,
    allow boolean DEFAULT true NOT NULL,
    created_at timestamp with time zone DEFAULT now(),
    created_by bigint,
    updated_at timestamp with time zone,
    updated_by bigint
);


ALTER TABLE public.rbac_role_permissions OWNER TO postgres;

--
-- Name: rbac_role_permissions_id_seq; Type: SEQUENCE; Schema: public; Owner: postgres
--

CREATE SEQUENCE public.rbac_role_permissions_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER SEQUENCE public.rbac_role_permissions_id_seq OWNER TO postgres;

--
-- Name: rbac_role_permissions_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: postgres
--

ALTER SEQUENCE public.rbac_role_permissions_id_seq OWNED BY public.rbac_role_permissions.id;


--
-- Name: rbac_roles; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.rbac_roles (
    id bigint NOT NULL,
    name text NOT NULL,
    description text,
    is_active boolean DEFAULT true,
    created_at timestamp with time zone DEFAULT now(),
    created_by bigint,
    updated_at timestamp with time zone,
    updated_by bigint
);


ALTER TABLE public.rbac_roles OWNER TO postgres;

--
-- Name: rbac_roles_id_seq; Type: SEQUENCE; Schema: public; Owner: postgres
--

CREATE SEQUENCE public.rbac_roles_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER SEQUENCE public.rbac_roles_id_seq OWNER TO postgres;

--
-- Name: rbac_roles_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: postgres
--

ALTER SEQUENCE public.rbac_roles_id_seq OWNED BY public.rbac_roles.id;


--
-- Name: rbac_submodules; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.rbac_submodules (
    id bigint NOT NULL,
    module_id bigint NOT NULL,
    code text NOT NULL,
    name text,
    meta jsonb,
    is_active boolean DEFAULT true,
    created_at timestamp with time zone DEFAULT now(),
    created_by bigint,
    updated_at timestamp with time zone,
    updated_by bigint
);


ALTER TABLE public.rbac_submodules OWNER TO postgres;

--
-- Name: rbac_submodules_id_seq; Type: SEQUENCE; Schema: public; Owner: postgres
--

CREATE SEQUENCE public.rbac_submodules_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER SEQUENCE public.rbac_submodules_id_seq OWNER TO postgres;

--
-- Name: rbac_submodules_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: postgres
--

ALTER SEQUENCE public.rbac_submodules_id_seq OWNED BY public.rbac_submodules.id;


--
-- Name: rbac_user_permissions_override; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.rbac_user_permissions_override (
    id bigint NOT NULL,
    user_id bigint NOT NULL,
    module_id bigint NOT NULL,
    submodule_id bigint,
    permission_type_id bigint NOT NULL,
    allow boolean NOT NULL,
    created_at timestamp with time zone DEFAULT now(),
    created_by bigint,
    updated_at timestamp with time zone,
    updated_by bigint
);


ALTER TABLE public.rbac_user_permissions_override OWNER TO postgres;

--
-- Name: rbac_user_permissions_override_id_seq; Type: SEQUENCE; Schema: public; Owner: postgres
--

CREATE SEQUENCE public.rbac_user_permissions_override_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER SEQUENCE public.rbac_user_permissions_override_id_seq OWNER TO postgres;

--
-- Name: rbac_user_permissions_override_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: postgres
--

ALTER SEQUENCE public.rbac_user_permissions_override_id_seq OWNED BY public.rbac_user_permissions_override.id;


--
-- Name: rbac_user_roles; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.rbac_user_roles (
    id bigint NOT NULL,
    user_id bigint NOT NULL,
    role_id bigint NOT NULL,
    assigned_by bigint,
    created_at timestamp with time zone DEFAULT now(),
    updated_at timestamp with time zone,
    updated_by bigint
);


ALTER TABLE public.rbac_user_roles OWNER TO postgres;

--
-- Name: rbac_user_roles_id_seq; Type: SEQUENCE; Schema: public; Owner: postgres
--

CREATE SEQUENCE public.rbac_user_roles_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER SEQUENCE public.rbac_user_roles_id_seq OWNER TO postgres;

--
-- Name: rbac_user_roles_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: postgres
--

ALTER SEQUENCE public.rbac_user_roles_id_seq OWNED BY public.rbac_user_roles.id;


--
-- Name: sessions; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.sessions (
    id bigint NOT NULL,
    user_id bigint,
    auth_token text NOT NULL,
    device_id text,
    ip_address text,
    user_agent text,
    geo_location text,
    device_metadata jsonb,
    is_active boolean DEFAULT true,
    last_seen_at timestamp with time zone,
    created_at timestamp with time zone DEFAULT now(),
    updated_at timestamp with time zone DEFAULT now(),
    is_single_use boolean DEFAULT false,
    is_temp boolean DEFAULT false,
    is_used boolean DEFAULT false,
    purpose text,
    expires_at timestamp with time zone
);


ALTER TABLE public.sessions OWNER TO postgres;

--
-- Name: user_otps; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.user_otps (
    id bigint NOT NULL,
    user_id bigint,
    code text NOT NULL,
    channel text NOT NULL,
    purpose text NOT NULL,
    issued_at timestamp with time zone DEFAULT now(),
    valid_until timestamp with time zone NOT NULL,
    is_verified boolean DEFAULT false,
    is_active boolean DEFAULT true,
    updated_at timestamp with time zone DEFAULT now()
);


ALTER TABLE public.user_otps OWNER TO postgres;

--
-- Name: user_preferences; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.user_preferences (
    user_id bigint NOT NULL,
    preferences jsonb DEFAULT '{}'::jsonb NOT NULL,
    updated_at timestamp with time zone DEFAULT now()
);


ALTER TABLE public.user_preferences OWNER TO postgres;

--
-- Name: user_profiles; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.user_profiles (
    user_id bigint NOT NULL,
    date_of_birth date,
    profile_image_url text,
    first_name character varying(100),
    last_name character varying(100),
    surname character varying(100),
    sys_username character varying(100),
    address jsonb,
    gender text,
    bio text,
    created_at timestamp with time zone DEFAULT now(),
    updated_at timestamp with time zone DEFAULT now(),
    nationality character(2)
);


ALTER TABLE public.user_profiles OWNER TO postgres;

--
-- Name: user_twofa; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.user_twofa (
    id bigint NOT NULL,
    user_id bigint NOT NULL,
    method text NOT NULL,
    secret text,
    is_enabled boolean DEFAULT false,
    created_at timestamp with time zone DEFAULT now(),
    updated_at timestamp with time zone DEFAULT now()
);


ALTER TABLE public.user_twofa OWNER TO postgres;

--
-- Name: user_twofa_backup_codes; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.user_twofa_backup_codes (
    id bigint NOT NULL,
    twofa_id bigint NOT NULL,
    code_hash text NOT NULL,
    is_used boolean DEFAULT false,
    created_at timestamp with time zone DEFAULT now(),
    used_at timestamp with time zone
);


ALTER TABLE public.user_twofa_backup_codes OWNER TO postgres;

--
-- Name: user_twofa_backup_codes_id_seq; Type: SEQUENCE; Schema: public; Owner: postgres
--

CREATE SEQUENCE public.user_twofa_backup_codes_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER SEQUENCE public.user_twofa_backup_codes_id_seq OWNER TO postgres;

--
-- Name: user_twofa_backup_codes_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: postgres
--

ALTER SEQUENCE public.user_twofa_backup_codes_id_seq OWNED BY public.user_twofa_backup_codes.id;


--
-- Name: user_twofa_id_seq; Type: SEQUENCE; Schema: public; Owner: postgres
--

CREATE SEQUENCE public.user_twofa_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER SEQUENCE public.user_twofa_id_seq OWNER TO postgres;

--
-- Name: user_twofa_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: postgres
--

ALTER SEQUENCE public.user_twofa_id_seq OWNED BY public.user_twofa.id;


--
-- Name: users; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.users (
    id bigint NOT NULL,
    email public.citext,
    phone character varying(20),
    password_hash text,
    first_name character varying(100),
    last_name character varying(100),
    is_email_verified boolean DEFAULT false,
    is_phone_verified boolean DEFAULT false,
    signup_stage text DEFAULT 'email_or_phone_submitted'::text,
    account_status text DEFAULT 'active'::text,
    account_type text DEFAULT 'password'::text,
    account_restored boolean DEFAULT false,
    created_at timestamp with time zone DEFAULT now(),
    updated_at timestamp with time zone DEFAULT now(),
    consent boolean DEFAULT true NOT NULL,
    pending_email text,
    pending_email_expires_at timestamp with time zone,
    changed_emails jsonb,
    changed_phones jsonb,
    CONSTRAINT account_type_check CHECK ((account_type = ANY (ARRAY['password'::text, 'social'::text, 'hybrid'::text]))),
    CONSTRAINT signup_stage_check CHECK ((signup_stage = ANY (ARRAY['email_or_phone_submitted'::text, 'otp_verified'::text, 'password_set'::text, 'complete'::text]))),
    CONSTRAINT users_contact_check CHECK (((email IS NOT NULL) OR (phone IS NOT NULL)))
);


ALTER TABLE public.users OWNER TO postgres;

--
-- Name: COLUMN users.consent; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.users.consent IS 'User agrees to terms and conditions';


--
-- Name: COLUMN users.pending_email; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.users.pending_email IS 'User submits new email awaiting verification';


--
-- Name: COLUMN users.pending_email_expires_at; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.users.pending_email_expires_at IS 'Expiry time for pending emails';


--
-- Name: COLUMN users.changed_emails; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.users.changed_emails IS 'changed user emails';


--
-- Name: wallet_transactions; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.wallet_transactions (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    wallet_id uuid NOT NULL,
    user_id text NOT NULL,
    currency character varying(10) NOT NULL,
    tx_status character varying(20) NOT NULL,
    amount numeric(20,8) NOT NULL,
    tx_type character varying(20) NOT NULL,
    description text,
    ref_id uuid,
    created_at timestamp without time zone DEFAULT now()
);


ALTER TABLE public.wallet_transactions OWNER TO postgres;

--
-- Name: wallets; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.wallets (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    user_id text NOT NULL,
    currency character varying(10) NOT NULL,
    balance numeric(20,8) DEFAULT 0,
    available numeric(20,8) DEFAULT 0,
    locked numeric(20,8) DEFAULT 0,
    type character varying(20) NOT NULL,
    created_at timestamp without time zone DEFAULT now(),
    updated_at timestamp without time zone DEFAULT now()
);


ALTER TABLE public.wallets OWNER TO postgres;

--
-- Name: countries id; Type: DEFAULT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.countries ALTER COLUMN id SET DEFAULT nextval('public.countries_id_seq'::regclass);


--
-- Name: kyc_audit_logs id; Type: DEFAULT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.kyc_audit_logs ALTER COLUMN id SET DEFAULT nextval('public.kyc_audit_logs_id_seq'::regclass);


--
-- Name: kyc_submissions id; Type: DEFAULT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.kyc_submissions ALTER COLUMN id SET DEFAULT nextval('public.kyc_submissions_id_seq'::regclass);


--
-- Name: partner_audit_logs id; Type: DEFAULT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.partner_audit_logs ALTER COLUMN id SET DEFAULT nextval('public.partner_audit_logs_id_seq'::regclass);


--
-- Name: rbac_modules id; Type: DEFAULT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.rbac_modules ALTER COLUMN id SET DEFAULT nextval('public.rbac_modules_id_seq'::regclass);


--
-- Name: rbac_permission_types id; Type: DEFAULT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.rbac_permission_types ALTER COLUMN id SET DEFAULT nextval('public.rbac_permission_types_id_seq'::regclass);


--
-- Name: rbac_permissions_audit id; Type: DEFAULT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.rbac_permissions_audit ALTER COLUMN id SET DEFAULT nextval('public.rbac_permissions_audit_id_seq'::regclass);


--
-- Name: rbac_role_permissions id; Type: DEFAULT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.rbac_role_permissions ALTER COLUMN id SET DEFAULT nextval('public.rbac_role_permissions_id_seq'::regclass);


--
-- Name: rbac_roles id; Type: DEFAULT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.rbac_roles ALTER COLUMN id SET DEFAULT nextval('public.rbac_roles_id_seq'::regclass);


--
-- Name: rbac_submodules id; Type: DEFAULT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.rbac_submodules ALTER COLUMN id SET DEFAULT nextval('public.rbac_submodules_id_seq'::regclass);


--
-- Name: rbac_user_permissions_override id; Type: DEFAULT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.rbac_user_permissions_override ALTER COLUMN id SET DEFAULT nextval('public.rbac_user_permissions_override_id_seq'::regclass);


--
-- Name: rbac_user_roles id; Type: DEFAULT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.rbac_user_roles ALTER COLUMN id SET DEFAULT nextval('public.rbac_user_roles_id_seq'::regclass);


--
-- Name: user_twofa id; Type: DEFAULT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.user_twofa ALTER COLUMN id SET DEFAULT nextval('public.user_twofa_id_seq'::regclass);


--
-- Name: user_twofa_backup_codes id; Type: DEFAULT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.user_twofa_backup_codes ALTER COLUMN id SET DEFAULT nextval('public.user_twofa_backup_codes_id_seq'::regclass);


--
-- Name: account_deletion_requests account_deletion_requests_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.account_deletion_requests
    ADD CONSTRAINT account_deletion_requests_pkey PRIMARY KEY (id);


--
-- Name: countries countries_iso2_key; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.countries
    ADD CONSTRAINT countries_iso2_key UNIQUE (iso2);


--
-- Name: countries countries_iso3_key; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.countries
    ADD CONSTRAINT countries_iso3_key UNIQUE (iso3);


--
-- Name: countries countries_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.countries
    ADD CONSTRAINT countries_pkey PRIMARY KEY (id);


--
-- Name: email_logs email_logs_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.email_logs
    ADD CONSTRAINT email_logs_pkey PRIMARY KEY (id);


--
-- Name: kyc_audit_logs kyc_audit_logs_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.kyc_audit_logs
    ADD CONSTRAINT kyc_audit_logs_pkey PRIMARY KEY (id);


--
-- Name: kyc_submissions kyc_submissions_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.kyc_submissions
    ADD CONSTRAINT kyc_submissions_pkey PRIMARY KEY (id);


--
-- Name: oauth_accounts oauth_accounts_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.oauth_accounts
    ADD CONSTRAINT oauth_accounts_pkey PRIMARY KEY (id);


--
-- Name: partner_audit_logs partner_audit_logs_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.partner_audit_logs
    ADD CONSTRAINT partner_audit_logs_pkey PRIMARY KEY (id);


--
-- Name: partner_configs partner_configs_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.partner_configs
    ADD CONSTRAINT partner_configs_pkey PRIMARY KEY (partner_id);


--
-- Name: partner_kyc partner_kyc_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.partner_kyc
    ADD CONSTRAINT partner_kyc_pkey PRIMARY KEY (partner_id);


--
-- Name: partner_users partner_users_partner_id_user_id_key; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.partner_users
    ADD CONSTRAINT partner_users_partner_id_user_id_key UNIQUE (partner_id, user_id);


--
-- Name: partner_users partner_users_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.partner_users
    ADD CONSTRAINT partner_users_pkey PRIMARY KEY (id);


--
-- Name: partners partners_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.partners
    ADD CONSTRAINT partners_pkey PRIMARY KEY (id);


--
-- Name: rbac_modules rbac_modules_code_key; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.rbac_modules
    ADD CONSTRAINT rbac_modules_code_key UNIQUE (code);


--
-- Name: rbac_modules rbac_modules_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.rbac_modules
    ADD CONSTRAINT rbac_modules_pkey PRIMARY KEY (id);


--
-- Name: rbac_permission_types rbac_permission_types_code_key; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.rbac_permission_types
    ADD CONSTRAINT rbac_permission_types_code_key UNIQUE (code);


--
-- Name: rbac_permission_types rbac_permission_types_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.rbac_permission_types
    ADD CONSTRAINT rbac_permission_types_pkey PRIMARY KEY (id);


--
-- Name: rbac_permissions_audit rbac_permissions_audit_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.rbac_permissions_audit
    ADD CONSTRAINT rbac_permissions_audit_pkey PRIMARY KEY (id);


--
-- Name: rbac_role_permissions rbac_role_permissions_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.rbac_role_permissions
    ADD CONSTRAINT rbac_role_permissions_pkey PRIMARY KEY (id);


--
-- Name: rbac_role_permissions rbac_role_permissions_role_id_module_id_submodule_id_permis_key; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.rbac_role_permissions
    ADD CONSTRAINT rbac_role_permissions_role_id_module_id_submodule_id_permis_key UNIQUE (role_id, module_id, submodule_id, permission_type_id);


--
-- Name: rbac_roles rbac_roles_name_key; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.rbac_roles
    ADD CONSTRAINT rbac_roles_name_key UNIQUE (name);


--
-- Name: rbac_roles rbac_roles_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.rbac_roles
    ADD CONSTRAINT rbac_roles_pkey PRIMARY KEY (id);


--
-- Name: rbac_submodules rbac_submodules_module_id_code_key; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.rbac_submodules
    ADD CONSTRAINT rbac_submodules_module_id_code_key UNIQUE (module_id, code);


--
-- Name: rbac_submodules rbac_submodules_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.rbac_submodules
    ADD CONSTRAINT rbac_submodules_pkey PRIMARY KEY (id);


--
-- Name: rbac_user_permissions_override rbac_user_permissions_overrid_user_id_module_id_submodule_i_key; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.rbac_user_permissions_override
    ADD CONSTRAINT rbac_user_permissions_overrid_user_id_module_id_submodule_i_key UNIQUE (user_id, module_id, submodule_id, permission_type_id);


--
-- Name: rbac_user_permissions_override rbac_user_permissions_override_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.rbac_user_permissions_override
    ADD CONSTRAINT rbac_user_permissions_override_pkey PRIMARY KEY (id);


--
-- Name: rbac_user_roles rbac_user_roles_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.rbac_user_roles
    ADD CONSTRAINT rbac_user_roles_pkey PRIMARY KEY (id);


--
-- Name: sessions sessions_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.sessions
    ADD CONSTRAINT sessions_pkey PRIMARY KEY (id);


--
-- Name: sessions unique_user_device_type; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.sessions
    ADD CONSTRAINT unique_user_device_type UNIQUE (user_id, device_id, is_temp);


--
-- Name: user_twofa unique_user_method; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.user_twofa
    ADD CONSTRAINT unique_user_method UNIQUE (user_id, method);


--
-- Name: rbac_user_roles uq_user_role; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.rbac_user_roles
    ADD CONSTRAINT uq_user_role UNIQUE (user_id, role_id);


--
-- Name: user_otps user_otps_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.user_otps
    ADD CONSTRAINT user_otps_pkey PRIMARY KEY (id);


--
-- Name: user_preferences user_preferences_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.user_preferences
    ADD CONSTRAINT user_preferences_pkey PRIMARY KEY (user_id);


--
-- Name: user_profiles user_profiles_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.user_profiles
    ADD CONSTRAINT user_profiles_pkey PRIMARY KEY (user_id);


--
-- Name: user_twofa_backup_codes user_twofa_backup_codes_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.user_twofa_backup_codes
    ADD CONSTRAINT user_twofa_backup_codes_pkey PRIMARY KEY (id);


--
-- Name: user_twofa user_twofa_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.user_twofa
    ADD CONSTRAINT user_twofa_pkey PRIMARY KEY (id);


--
-- Name: users users_email_key; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.users
    ADD CONSTRAINT users_email_key UNIQUE (email);


--
-- Name: users users_phone_key; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.users
    ADD CONSTRAINT users_phone_key UNIQUE (phone);


--
-- Name: users users_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.users
    ADD CONSTRAINT users_pkey PRIMARY KEY (id);


--
-- Name: wallet_transactions wallet_transactions_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.wallet_transactions
    ADD CONSTRAINT wallet_transactions_pkey PRIMARY KEY (id);


--
-- Name: wallets wallets_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.wallets
    ADD CONSTRAINT wallets_pkey PRIMARY KEY (id);


--
-- Name: idx_account_deletion_requested_at; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_account_deletion_requested_at ON public.account_deletion_requests USING btree (requested_at);


--
-- Name: idx_account_deletion_user_id; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_account_deletion_user_id ON public.account_deletion_requests USING btree (user_id);


--
-- Name: idx_audit_logs_action; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_audit_logs_action ON public.partner_audit_logs USING btree (action);


--
-- Name: idx_audit_logs_actor; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_audit_logs_actor ON public.partner_audit_logs USING btree (actor_type, actor_id);


--
-- Name: idx_backup_codes_is_used; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_backup_codes_is_used ON public.user_twofa_backup_codes USING btree (is_used);


--
-- Name: idx_backup_codes_twofa_id; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_backup_codes_twofa_id ON public.user_twofa_backup_codes USING btree (twofa_id);


--
-- Name: idx_email_logs_status; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_email_logs_status ON public.email_logs USING btree (status);


--
-- Name: idx_email_logs_type; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_email_logs_type ON public.email_logs USING btree (type);


--
-- Name: idx_email_logs_user_id; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_email_logs_user_id ON public.email_logs USING btree (user_id);


--
-- Name: idx_kyc_logs_kyc_id; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_kyc_logs_kyc_id ON public.kyc_audit_logs USING btree (kyc_id);


--
-- Name: idx_kyc_status; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_kyc_status ON public.kyc_submissions USING btree (status);


--
-- Name: idx_kyc_user; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_kyc_user ON public.kyc_submissions USING btree (user_id);


--
-- Name: idx_oauth_accounts_provider_uid; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_oauth_accounts_provider_uid ON public.oauth_accounts USING btree (provider_uid);


--
-- Name: idx_oauth_accounts_user_id; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_oauth_accounts_user_id ON public.oauth_accounts USING btree (user_id);


--
-- Name: idx_partner_users_partner_id; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_partner_users_partner_id ON public.partner_users USING btree (partner_id);


--
-- Name: idx_partners_name; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_partners_name ON public.partners USING btree (name);


--
-- Name: idx_sessions_is_active; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_sessions_is_active ON public.sessions USING btree (is_active);


--
-- Name: idx_sessions_token; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_sessions_token ON public.sessions USING btree (auth_token);


--
-- Name: idx_sessions_user_id; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_sessions_user_id ON public.sessions USING btree (user_id);


--
-- Name: idx_user_currency; Type: INDEX; Schema: public; Owner: postgres
--

CREATE UNIQUE INDEX idx_user_currency ON public.wallets USING btree (user_id, currency);


--
-- Name: idx_user_otps_channel; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_user_otps_channel ON public.user_otps USING btree (channel);


--
-- Name: idx_user_otps_code; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_user_otps_code ON public.user_otps USING btree (code);


--
-- Name: idx_user_otps_issued_at; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_user_otps_issued_at ON public.user_otps USING btree (issued_at);


--
-- Name: idx_user_otps_purpose; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_user_otps_purpose ON public.user_otps USING btree (purpose);


--
-- Name: idx_user_otps_user_id; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_user_otps_user_id ON public.user_otps USING btree (user_id);


--
-- Name: idx_user_preferences_gin; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_user_preferences_gin ON public.user_preferences USING gin (preferences jsonb_path_ops);


--
-- Name: idx_user_profiles_address_gin; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_user_profiles_address_gin ON public.user_profiles USING gin (address jsonb_path_ops);


--
-- Name: idx_user_profiles_dob; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_user_profiles_dob ON public.user_profiles USING btree (date_of_birth);


--
-- Name: idx_user_profiles_gender; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_user_profiles_gender ON public.user_profiles USING btree (gender);


--
-- Name: idx_user_twofa_method; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_user_twofa_method ON public.user_twofa USING btree (method);


--
-- Name: idx_user_twofa_user_id; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_user_twofa_user_id ON public.user_twofa USING btree (user_id);


--
-- Name: idx_users_account_status; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_users_account_status ON public.users USING btree (account_status);


--
-- Name: idx_users_account_type; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_users_account_type ON public.users USING btree (account_type);


--
-- Name: idx_users_created_at; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_users_created_at ON public.users USING btree (created_at);


--
-- Name: idx_users_email; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_users_email ON public.users USING btree (email);


--
-- Name: idx_users_phone; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_users_phone ON public.users USING btree (phone);


--
-- Name: uniq_role_module_permission; Type: INDEX; Schema: public; Owner: postgres
--

CREATE UNIQUE INDEX uniq_role_module_permission ON public.rbac_role_permissions USING btree (role_id, module_id, permission_type_id) WHERE (submodule_id IS NULL);


--
-- Name: uniq_role_module_submodule_permission; Type: INDEX; Schema: public; Owner: postgres
--

CREATE UNIQUE INDEX uniq_role_module_submodule_permission ON public.rbac_role_permissions USING btree (role_id, module_id, submodule_id, permission_type_id) WHERE (submodule_id IS NOT NULL);


--
-- Name: sessions trg_sessions_set_updated_at; Type: TRIGGER; Schema: public; Owner: postgres
--

CREATE TRIGGER trg_sessions_set_updated_at BEFORE UPDATE ON public.sessions FOR EACH ROW EXECUTE FUNCTION public.set_updated_at();


--
-- Name: kyc_submissions trg_set_updated_at; Type: TRIGGER; Schema: public; Owner: postgres
--

CREATE TRIGGER trg_set_updated_at BEFORE UPDATE ON public.kyc_submissions FOR EACH ROW EXECUTE FUNCTION public.set_updated_at();


--
-- Name: user_otps trg_user_otps_set_updated_at; Type: TRIGGER; Schema: public; Owner: postgres
--

CREATE TRIGGER trg_user_otps_set_updated_at BEFORE UPDATE ON public.user_otps FOR EACH ROW EXECUTE FUNCTION public.set_updated_at();


--
-- Name: users trg_users_set_updated_at; Type: TRIGGER; Schema: public; Owner: postgres
--

CREATE TRIGGER trg_users_set_updated_at BEFORE UPDATE ON public.users FOR EACH ROW EXECUTE FUNCTION public.set_updated_at();


--
-- Name: account_deletion_requests account_deletion_requests_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.account_deletion_requests
    ADD CONSTRAINT account_deletion_requests_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: kyc_audit_logs fk_kyc; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.kyc_audit_logs
    ADD CONSTRAINT fk_kyc FOREIGN KEY (kyc_id) REFERENCES public.kyc_submissions(id) ON DELETE CASCADE;


--
-- Name: kyc_submissions fk_kyc_user; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.kyc_submissions
    ADD CONSTRAINT fk_kyc_user FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: user_preferences fk_user_preferences_user; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.user_preferences
    ADD CONSTRAINT fk_user_preferences_user FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: user_profiles fk_user_profiles_country; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.user_profiles
    ADD CONSTRAINT fk_user_profiles_country FOREIGN KEY (nationality) REFERENCES public.countries(iso2) ON DELETE SET NULL;


--
-- Name: user_profiles fk_user_profiles_user; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.user_profiles
    ADD CONSTRAINT fk_user_profiles_user FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: user_twofa fk_user_twofa_user; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.user_twofa
    ADD CONSTRAINT fk_user_twofa_user FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: oauth_accounts oauth_accounts_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.oauth_accounts
    ADD CONSTRAINT oauth_accounts_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: partner_configs partner_configs_partner_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.partner_configs
    ADD CONSTRAINT partner_configs_partner_id_fkey FOREIGN KEY (partner_id) REFERENCES public.partners(id) ON DELETE CASCADE;


--
-- Name: partner_kyc partner_kyc_partner_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.partner_kyc
    ADD CONSTRAINT partner_kyc_partner_id_fkey FOREIGN KEY (partner_id) REFERENCES public.partners(id) ON DELETE CASCADE;


--
-- Name: partner_users partner_users_partner_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.partner_users
    ADD CONSTRAINT partner_users_partner_id_fkey FOREIGN KEY (partner_id) REFERENCES public.partners(id) ON DELETE CASCADE;


--
-- Name: partner_users partner_users_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.partner_users
    ADD CONSTRAINT partner_users_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: rbac_modules rbac_modules_parent_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.rbac_modules
    ADD CONSTRAINT rbac_modules_parent_id_fkey FOREIGN KEY (parent_id) REFERENCES public.rbac_modules(id) ON DELETE CASCADE;


--
-- Name: rbac_role_permissions rbac_role_permissions_module_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.rbac_role_permissions
    ADD CONSTRAINT rbac_role_permissions_module_id_fkey FOREIGN KEY (module_id) REFERENCES public.rbac_modules(id) ON DELETE CASCADE;


--
-- Name: rbac_role_permissions rbac_role_permissions_permission_type_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.rbac_role_permissions
    ADD CONSTRAINT rbac_role_permissions_permission_type_id_fkey FOREIGN KEY (permission_type_id) REFERENCES public.rbac_permission_types(id);


--
-- Name: rbac_role_permissions rbac_role_permissions_role_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.rbac_role_permissions
    ADD CONSTRAINT rbac_role_permissions_role_id_fkey FOREIGN KEY (role_id) REFERENCES public.rbac_roles(id) ON DELETE CASCADE;


--
-- Name: rbac_role_permissions rbac_role_permissions_submodule_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.rbac_role_permissions
    ADD CONSTRAINT rbac_role_permissions_submodule_id_fkey FOREIGN KEY (submodule_id) REFERENCES public.rbac_submodules(id) ON DELETE CASCADE;


--
-- Name: rbac_submodules rbac_submodules_module_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.rbac_submodules
    ADD CONSTRAINT rbac_submodules_module_id_fkey FOREIGN KEY (module_id) REFERENCES public.rbac_modules(id) ON DELETE CASCADE;


--
-- Name: rbac_user_permissions_override rbac_user_permissions_override_module_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.rbac_user_permissions_override
    ADD CONSTRAINT rbac_user_permissions_override_module_id_fkey FOREIGN KEY (module_id) REFERENCES public.rbac_modules(id) ON DELETE CASCADE;


--
-- Name: rbac_user_permissions_override rbac_user_permissions_override_permission_type_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.rbac_user_permissions_override
    ADD CONSTRAINT rbac_user_permissions_override_permission_type_id_fkey FOREIGN KEY (permission_type_id) REFERENCES public.rbac_permission_types(id) ON DELETE CASCADE;


--
-- Name: rbac_user_permissions_override rbac_user_permissions_override_submodule_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.rbac_user_permissions_override
    ADD CONSTRAINT rbac_user_permissions_override_submodule_id_fkey FOREIGN KEY (submodule_id) REFERENCES public.rbac_submodules(id) ON DELETE CASCADE;


--
-- Name: rbac_user_permissions_override rbac_user_permissions_override_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.rbac_user_permissions_override
    ADD CONSTRAINT rbac_user_permissions_override_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: rbac_user_roles rbac_user_roles_role_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.rbac_user_roles
    ADD CONSTRAINT rbac_user_roles_role_id_fkey FOREIGN KEY (role_id) REFERENCES public.rbac_roles(id) ON DELETE CASCADE;


--
-- Name: rbac_user_roles rbac_user_roles_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.rbac_user_roles
    ADD CONSTRAINT rbac_user_roles_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: sessions sessions_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.sessions
    ADD CONSTRAINT sessions_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: user_otps user_otps_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.user_otps
    ADD CONSTRAINT user_otps_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE SET NULL;


--
-- Name: user_twofa_backup_codes user_twofa_backup_codes_twofa_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.user_twofa_backup_codes
    ADD CONSTRAINT user_twofa_backup_codes_twofa_id_fkey FOREIGN KEY (twofa_id) REFERENCES public.user_twofa(id) ON DELETE CASCADE;


--
-- PostgreSQL database dump complete
--

