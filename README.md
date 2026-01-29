**Project X scalable microservices project structure in Go**, with support for:

* gRPC communication between services
* Docker for containerization
* Kubernetes for orchestration
* Independent deployment per service
* Easy addition of new services later

---

## Microservices Project Structure (Clean, Scalable & Extendable)

```
\pxyz
|   docker-compose.prod.yaml
|   docker-compose.yaml
|   prometheus.yml
|   README.md
|
+---docker compose
|       docker-compose.user.auth.db.yaml
|       docker-compose.yaml
|       wait-for-replicas.sh
|
+---k8s
|   |   00-namespace.yaml
|   |   Makefile
|   |   ReadMe.md
|   |
|   +---01-storage
|   |       kafka-pv.yaml
|   |       redis-pv.yaml
|   |       storage-class.yaml
|   |       uploads-pv.yaml
|   |       zookeeper-pv.yaml
|   |
|   +---02-secrets
|   |       auth-secrets.yaml
|   |       db-secrets.yaml
|   |       email-secrets.yaml
|   |       jwt-secrets.yaml
|   |       kafka-secrets.yaml
|   |       redis-secrete.yaml
|   |       sms-secret.yaml
|   |
|   +---03-configmaps
|   |       auth-config.yaml
|   |       common-config.yaml
|   |       kafka-config.yaml
|   |       redis-config.yaml
|   |
|   +---04-infrastructure
|   |       kafka.yaml
|   |       redis.yaml
|   |       zookeeper.yaml
|   |
|   +---05-services
|   |       account-service.yaml
|   |       audit-service.yaml
|   |       auth-service.yaml
|   |       core-service.yaml
|   |       email-service.yaml
|   |       kyc-service.yaml
|   |       notification-service.yaml
|   |       otp-service.yaml
|   |       session-service.yaml
|   |       sms-service.yaml
|   |       u-access-service.yaml
|   |
|   +---05-services-new
|   |       account-service.yaml
|   |       audit-service.yaml
|   |       auth-service.yaml
|   |       core-service.yaml
|   |       email-service.yaml
|   |       kyc-service.yaml
|   |       notification-service.yaml
|   |       otp-service.yaml
|   |       session-service.yaml
|   |       sms-service.yaml
|   |       u-access-service.yaml
|   |
|   +---06-autoscaling
|   |       hpa.yaml
|   |
|   +---07-ingress
|   |       ingress.yaml
|   |
|   +---07-ingress-new
|   |       ingress.yaml
|   |
|   \---scripts
|           build-images.sh
|           check-k8s-requirements.sh
|           cleanup.sh
|           del-deploy.sh
|           deploy-k8s.sh
|           install-k8s.sh
|           kafka-perm.sh
|           status.sh
|           uninstall.sh
|
+---services
|   +---admin-services
|   |   +---admin-auth-service
|   |   |   |   .env
|   |   |   |   Dockerfile
|   |   |   |   go.mod
|   |   |   |   go.sum
|   |   |   |
|   |   |   +---cmd
|   |   |   |       auth.main.go
|   |   |   |
|   |   |   +---db
|   |   |   |       init.sql
|   |   |   |
|   |   |   +---internal
|   |   |   |   +---config
|   |   |   |   |       auth.config.go
|   |   |   |   |       auth_db.config.go
|   |   |   |   |
|   |   |   |   +---domain
|   |   |   |   |       account_deletion.domain.go
|   |   |   |   |       ptn_domain.go
|   |   |   |   |       session.domain.go
|   |   |   |   |       user.domain.go
|   |   |   |   |
|   |   |   |   +---handler
|   |   |   |   |       auth.main.handler.go
|   |   |   |   |       auth_2fa.handler.go
|   |   |   |   |       auth_login.handler.go
|   |   |   |   |       auth_otp.handler.go
|   |   |   |   |       auth_profile.handler.go
|   |   |   |   |       auth_register.handler.go
|   |   |   |   |       auth_req_body.handler.go
|   |   |   |   |       auth_session.handler.go
|   |   |   |   |       auth_session_helper.go
|   |   |   |   |       auth_update.handler.go
|   |   |   |   |       auth_ws.handler.go
|   |   |   |   |       grpc.handler.go
|   |   |   |   |       helper_func.go
|   |   |   |   |       otp.utils.go
|   |   |   |
|   |   |   |   +---repository
|   |   |   |   |       ptn_repo.go
|   |   |   |   |       user.repository.go
|   |   |   |   |       user_login.repo.go
|   |   |   |   |       user_register.repo.go
|   |   |   |   |
|   |   |   |   +---router
|   |   |   |   |       auth.router.go
|   |   |   |   |
|   |   |   |   +---server
|   |   |   |   |       auth.server.go
|   |   |   |   |
|   |   |   |   +---usecase
|   |   |   |   |       auth.main.usecase.go
|   |   |   |   |       auth_login.usecase.go
|   |   |   |   |       auth_profile.usecase.go
|   |   |   |   |       auth_register.usecase.go
|   |   |   |   |       helper_func.go
|   |   |   |   |       ptn_usecase.go
|   |   |   |   |
|   |   |   |   \---ws
|   |   |   |           client.go
|   |   |   |           events.go
|   |   |   |           hub.go
|   |   |   |           message.go
|   |   |   |           server.go
|   |   |   |
|   |   |   +---pkg
|   |   |   |   \---utils
|   |   |   |       |   auth_utils.go
|   |   |   |       |   auth_validation.go
|   |   |   |       |
|   |   |   |       \---errors
|   |   |   |               auth.errors.go
|   |   |   |
|   |   |   \---secrets
|   |   |           jwt_public.pem
|   |   |
|   |   +---admin-service
|   |   +---admin-session-mngt
|   |   \---u-access-service
|   |
|   +---common-services
|   |   +---authentication
|   |   +---comms-services
|   |   +---core-service
|   |   \---fx-services
|   |
|   +---partner-services
|   |   +---partner-service
|   |   |   |   .env
|   |   |   |   Dockerfile
|   |   |   |   go.mod
|   |   |   |   go.sum
|   |   |   |
|   |   |   +---cmd
|   |   |   |       ptn.main.go
|   |   |   |
|   |   |   +---db
|   |   |   |       partner.sql
|   |   |   |       ptn_schema.sql
|   |   |   |
|   |   |   +---internal
|   |   |   |   +---config
|   |   |   |   |       ptn.config.go
|   |   |   |   |       ptn_db.config.go
|   |   |   |   |
|   |   |   |   +---domain
|   |   |   |   |       ptn.domain.go
|   |   |   |   |       ptn_audit_log.domain.go
|   |   |   |   |       ptn_cf.domain.go
|   |   |   |   |       ptn_kyc.domain.go
|   |   |   |   |       user.domain.go
|   |   |   |   |
|   |   |   |   +---events
|   |   |   |   |       publisher.go
|   |   |   |   |
|   |   |   |   +---handler
|   |   |   |   |       api_fx_deposit.go
|   |   |   |   |       api_fx_withdraw.go
|   |   |   |   |       email.helper.go
|   |   |   |   |       grpc.partner_api.go
|   |   |   |   |       grpc.partner_transactions.go
|   |   |   |   |       grpc.partrner_webhooks.go
|   |   |   |   |       helper_func.go
|   |   |   |   |       ptn.grpc.handler.go
|   |   |   |   |       ptn.main.handler.go
|   |   |   |   |       ptn_fx.handler.go
|   |   |   |   |       rest.partner_api.go
|   |   |   |   |
|   |   |   |   +---repository
|   |   |   |   |       partner.go
|   |   |   |   |       ptn.repo.go
|   |   |   |   |       ptn_main.repo.go
|   |   |   |   |       ptn_user.repo.go
|   |   |   |   |       transactions.go
|   |   |   |   |
|   |   |   |   +---router
|   |   |   |   |       ptn.router.go
|   |   |   |   |
|   |   |   |   +---server
|   |   |   |   |       ptn.server.go
|   |   |   |   |
|   |   |   |   \---usecase
|   |   |   |           partner_api.go
|   |   |   |           partner_transactions.go
|   |   |   |           partner_webhooks.go
|   |   |   |           ptn.main.go
|   |   |   |           ptn.usecase.go
|   |   |   |
|   |   |   +---pkg
|   |   |   |   \---auth
|   |   |   |           auth.go
|   |   |   |           rate_limit.go
|   |   |   |
|   |   |   \---secrets
|   |   |           jwt_public.pem
|   |   |
|   |   +---ptn-auth-service
|   |   |   |   .env
|   |   |   |   Dockerfile
|   |   |   |   go.mod
|   |   |   |   go.sum
|   |   |   |
|   |   |   +---cmd
|   |   |   |       auth.main.go
|   |   |   |
|   |   |   +---db
|   |   |   |       init.sql
|   |   |   |
|   |   |   +---internal
|   |   |   |   +---config
|   |   |   |   |       auth.config.go
|   |   |   |   |       auth_db.config.go
|   |   |   |   |
|   |   |   |   +---domain
|   |   |   |   |       account_deletion.domain.go
|   |   |   |   |       ptn_domain.go
|   |   |   |   |       session.domain.go
|   |   |   |   |       user.domain.go
|   |   |   |   |
|   |   |   |   +---handler
|   |   |   |   |       auth.main.handler.go
|   |   |   |   |       auth_2fa.handler.go
|   |   |   |   |       auth_login.handler.go
|   |   |   |   |       auth_otp.handler.go
|   |   |   |   |       auth_profile.handler.go
|   |   |   |   |       auth_register.handler.go
|   |   |   |   |       auth_req_body.handler.go
|   |   |   |   |       auth_session.handler.go
|   |   |   |   |       auth_session_helper.go
|   |   |   |   |       auth_update.handler.go
|   |   |   |   |       auth_ws.handler.go
|   |   |   |   |       grpc.handler.go
|   |   |   |   |       helper_func.go
|   |   |   |   |       otp.utils.go
|   |   |   |   |
|   |   |   |   +---repository
|   |   |   |   |       ptn_repo.go
|   |   |   |   |       user.repository.go
|   |   |   |   |       user_login.repo.go
|   |   |   |   |       user_register.repo.go
|   |   |   |   |
|   |   |   |   +---router
|   |   |   |   |       auth.router.go
|   |   |   |   |
|   |   |   |   +---server
|   |   |   |   |       auth.server.go
|   |   |   |   |
|   |   |   |   +---usecase
|   |   |   |   |       auth.main.usecase.go
|   |   |   |   |       auth_login.usecase.go
|   |   |   |   |       auth_profile.usecase.go
|   |   |   |   |       auth_register.usecase.go
|   |   |   |   |       helper_func.go
|   |   |   |   |       ptn_usecase.go
|   |   |   |   |
|   |   |   |   \---ws
|   |   |   |           client.go
|   |   |   |           events.go
|   |   |   |           hub.go
|   |   |   |           message.go
|   |   |   |           server.go
|   |   |   |
|   |   |   +---pkg
|   |   |   |   \---utils
|   |   |   |       |   auth_utils.go
|   |   |   |       |   auth_validation.go
|   |   |   |       |
|   |   |   |       \---errors
|   |   |   |               auth.errors.go
|   |   |   |
|   |   |   \---secrets
|   |   |           jwt_public.pem
|   |   |
|   |   +---ptn-session-mngt
|   |   |   |   .env
|   |   |   |   Dockerfile
|   |   |   |   go.mod
|   |   |   |   go.sum
|   |   |   |
|   |   |   +---cmd
|   |   |   |       sess.main.go
|   |   |   |
|   |   |   +---internal
|   |   |   |   +---config
|   |   |   |   |       sess.config.go
|   |   |   |   |       sess_db.config.go
|   |   |   |   |
|   |   |   |   +---domain
|   |   |   |   |       session.domain.go
|   |   |   |   |       user.domain.go
|   |   |   |   |
|   |   |   |   +---handler
|   |   |   |   |       grpc.handler.go
|   |   |   |   |
|   |   |   |   +---repository
|   |   |   |   |       session.repository.go
|   |   |   |   |
|   |   |   |   \---usecase
|   |   |   |           session.usecase.go
|   |   |   |
|   |   |   +---pkg
|   |   |   |   \---jwtutil
|   |   |   |           generator.go
|   |   |   |           keys.go
|   |   |   |           loader.go
|   |   |   |
|   |   |   +---proto
|   |   |   |       auth.proto
|   |   |   |
|   |   |   \---secrets
|   |   |           jwt_private.pem
|   |   |           jwt_public.pem
|   |   |
|   |   \---u-access-service
|   |       |   .env
|   |       |   Dockerfile
|   |       |   go.mod
|   |       |   go.sum
|   |       |
|   |       +---cmd
|   |       |       u_access.main.go
|   |       |
|   |       +---db_scripts
|   |       |       access_db.sql
|   |       |
|   |       +---internal
|   |       |   +---config
|   |       |   |       u_access.config.go
|   |       |   |       u_access_db.config.go
|   |       |   |
|   |       |   +---domain
|   |       |   |       defaults.go
|   |       |   |       u_access.audit.domain.go
|   |       |   |       u_access.domain.go
|   |       |   |       u_access.module.domain.go
|   |       |   |       u_access.perm.domain.go
|   |       |   |       u_access.role.domain.go
|   |       |   |       u_access.role_perm.domain.go
|   |       |   |       u_access.subm.domain.go
|   |       |   |       u_access.user_perm_overide.domain.go
|   |       |   |       u_access.user_role.domain.go
|   |       |   |
|   |       |   +---handler
|   |       |   |   +---grpc
|   |       |   |   |       helper.go
|   |       |   |   |       mod.g.handler.go
|   |       |   |   |       roles.g.handler.go
|   |       |   |   |       user.g.go
|   |       |   |   |
|   |       |   |   \---rest
|   |       |   |           u_access.r.handler.go
|   |       |   |
|   |       |   +---repository
|   |       |   |       u_access.audit.repo.go
|   |       |   |       u_access.main.repo.go
|   |       |   |       u_access.mod.repo.go
|   |       |   |       u_access.perm.repo.go
|   |       |   |       u_access.roles.repo.go
|   |       |   |       u_access.user.repo.go
|   |       |   |
|   |       |   +---router
|   |       |   |       u_access.router.go
|   |       |   |
|   |       |   +---server
|   |       |   |       u_access.server.go
|   |       |   |
|   |       |   +---service
|   |       |   |       u_access_sync.go
|   |       |   |
|   |       |   \---usecase
|   |       |           u_access.uc.go
|   |       |
|   |       +---migrations
|   |       |       001_create_tbl.sql
|   |       |
|   |       +---proto
|   |       |       urbacpb.proto
|   |       |
|   |       \---secrets
|   |               jwt_public.pem
|   |
|   \---user-services
|       +---account-service
|       |   |   .env
|       |   |   Dockerfile
|       |   |   go.mod
|       |   |   go.sum
|       |   |
|       |   +---cmd
|       |   |       acc.main.go
|       |   |
|       |   +---db
|       |   |       account_mod.sql
|       |   |
|       |   \---internal
|       |       +---config
|       |       |       acc.config.go
|       |       |
|       |       +---domain
|       |       |       2fa.domain.go
|       |       |       acc.domain.go
|       |       |       prefs.domain.go
|       |       |
|       |       +---handler
|       |       |   +---2fa
|       |       |   |       2fa.handler.go
|       |       |   |
|       |       |   +---acc
|       |       |   |       acc.handler.go
|       |       |   |
|       |       |   \---prefs
|       |       |           prefs.handler.go
|       |       |
|       |       +---repository
|       |       |       2fa.repo.go
|       |       |       acc.repo.go
|       |       |       prefs.repo.go
|       |       |
|       |       +---server
|       |       |       acc.server.go
|       |       |
|       |       \---service
|       |           +---2fa
|       |           |       2fa.service.go
|       |           |       2fa.utils.go
|       |           |
|       |           +---acc
|       |           |       acc.service.go
|       |           |       acc.utils.go
|       |           |
|       |           \---prefs
|       |                   pref.service.go
|       |
|       +---cashier-service
|       |   |   .env
|       |   |   Dockerfile
|       |   |   go.mod
|       |   |   go.sum
|       |   |   READMe.md
|       |   |   READMe.pdf
|       |   |
|       |   +---cmd
|       |   |       csr.main.go
|       |   |
|       |   +---db
|       |   |       cashier.sql
|       |   |
|       |   +---internal
|       |   |   |   handler.zip
|       |   |   |
|       |   |   +---config
|       |   |   |       csr.config.go
|       |   |   |       csr_db.config.go
|       |   |   |
|       |   |   +---domain
|       |   |   |       csr.payment.domain.go
|       |   |   |       csr.provider.domain.go
|       |   |   |       csr.transaction.domain.go
|       |   |   |
|       |   |   +---event_handler
|       |   |   |       combined_event_handler.go
|       |   |   |       deposit_events.go
|       |   |   |       withdraw_events.go
|       |   |   |
|       |   |   +---handler
|       |   |   |       account.go
|       |   |   |       convert_transfer.go
|       |   |   |       crypto_helpers.go
|       |   |   |       csr.fx.handler.go
|       |   |   |       csr.main_handler.go
|       |   |   |       csr.ws.go
|       |   |   |       csr.ws_handler.go
|       |   |   |       deposit.go
|       |   |   |       deposit_agent.go
|       |   |   |       deposit_crypto.go
|       |   |   |       deposit_helpers.go
|       |   |   |       deposit_partner.go
|       |   |   |       deposit_validators.go
|       |   |   |       fee.go
|       |   |   |       helper_func.go
|       |   |   |       partner.go
|       |   |   |       report.go
|       |   |   |       transaction.go
|       |   |   |       transfer_funds.go
|       |   |   |       utils.go
|       |   |   |       withdraw.go
|       |   |   |       withdraw_agent.go
|       |   |   |       withdraw_crypto.go
|       |   |   |       withdraw_helpers.go
|       |   |   |       withdraw_partner.go
|       |   |   |       withdraw_validators.go
|       |   |   |       withdraw_verfication.go
|       |   |   |
|       |   |   +---provider
|       |   |   |   \---mpesa
|       |   |   |           b2c.mpesa.go
|       |   |   |           client.mpesa.go
|       |   |   |           mpesa.go
|       |   |   |           stk.mpesa.go
|       |   |   |
|       |   |   +---repository
|       |   |   |       trans_repo.go
|       |   |   |
|       |   |   +---router
|       |   |   |       csr.router.go
|       |   |   |
|       |   |   +---server
|       |   |   |       csr.server.go
|       |   |   |
|       |   |   +---service
|       |   |   |       convert.go
|       |   |   |
|       |   |   +---sub
|       |   |   |       partner_sub.go
|       |   |   |       subscriber.go
|       |   |   |
|       |   |   \---usecase
|       |   |       \---transaction
|       |   |               transaction.go
|       |   |
|       |   \---secrets
|       |           jwt_public.pem
|       |
|       \---kyc-service
|           |   .env
|           |   Dockerfile
|           |   go.mod
|           |   go.sum
|           |
|           +---cmd
|           |       kyc.main.go
|           |
|           +---db_scripts
|           |       db.sql
|           |
|           +---internal
|           |   +---config
|           |   |       kyc.config.go
|           |   |
|           |   +---domain
|           |   |       kyc.domain.go
|           |   |
|           |   +---handler
|           |   |       kyc.handler.go
|           |   |       utils.go
|           |   |
|           |   +---repository
|           |   |       kyc.repo.go
|           |   |
|           |   \---service
|           |           kyc.service.go
|           |
|           \---secrets
|                   jwt_public.pem
|
+---shared
|   |   .env
|   |   go.mod
|   |   go.sum
|   |   protoc
|   |
|   +---account
|   |       factory.account.go
|   |
|   +---auth
|   |   |   factory.auth.go
|   |   |
|   |   +---middleware
|   |   |       auth_middleware.go
|   |   |       context_utils.go
|   |   |       extractor.go
|   |   |       factory.go
|   |   |       rate.middleware.go
|   |   |
|   |   +---otp
|   |   |       otp.factory.go
|   |   |
|   |   \---pkg
|   |       \---jwtutil
|   |               jwt.go
|   |               keys.go
|   |               loader.go
|   |               verify.go
|   |
|   +---common
|   |   +---accounting
|   |   |       accounting.factory.go
|   |   |
|   |   +---crypto
|   |   |       crypto.factory.go
|   |   |
|   |   \---receipt
|   |           receipt_c_v2.factory.go
|   |
|   +---core
|   |       core.factory.go
|   |
|   +---email
|   |       factory.email.go
|   |
|   +---factory
|   |   +---admin
|   |   |   \---urbac
|   |   |       |   factory.urbac.go
|   |   |       |
|   |   |       \---utils
|   |   |               rbac_utils.go
|   |   |
|   |   \---partner
|   |       \---urbac
|   |           |   factory.urbac.go
|   |           |
|   |           \---utils
|   |                   rbac_utils.go
|   |
|   +---genproto
|   |   +---accountpb
|   |   |       account.pb.go
|   |   |       account_grpc.pb.go
|   |   |
|   |   +---admin
|   |   |   +---adminrbacpb
|   |   |   |       urbac.pb.go
|   |   |   |       urbac_grpc.pb.go
|   |   |   |
|   |   |   +---authpb
|   |   |   |       auth.pb.go
|   |   |   |       auth_grpc.pb.go
|   |   |   |
|   |   |   \---sessionpb
|   |   |           session.pb.go
|   |   |           session_grpc.pb.go
|   |   |
|   |   +---authentication
|   |   |   \---audit-service
|   |   |       \---grpcpb
|   |   |               audit.pb.go
|   |   |               audit_grpc.pb.go
|   |   |
|   |   +---authpb
|   |   |       auth.pb.go
|   |   |       auth_grpc.pb.go
|   |   |
|   |   +---corepb
|   |   |       core.pb.go
|   |   |       core_grpc.pb.go
|   |   |
|   |   +---emailpb
|   |   |       email.pb.go
|   |   |       email_grpc.pb.go
|   |   |
|   |   +---otppb
|   |   |       otp.pb.go
|   |   |       otp_grpc.pb.go
|   |   |
|   |   +---partner
|   |   |   +---authpb
|   |   |   |       auth.pb.go
|   |   |   |       auth_grpc.pb.go
|   |   |   |
|   |   |   +---ptnrbacpb
|   |   |   |       urbac.pb.go
|   |   |   |       urbac_grpc.pb.go
|   |   |   |
|   |   |   +---sessionpb
|   |   |   |       session.pb.go
|   |   |   |       session_grpc.pb.go
|   |   |   |
|   |   |   \---svcpb
|   |   |           svc.pb.go
|   |   |           svc_grpc.pb.go
|   |   |
|   |   +---sessionpb
|   |   |       session.pb.go
|   |   |       session_grpc.pb.go
|   |   |
|   |   +---shared
|   |   |   +---accounting
|   |   |   |   +---cryptopb
|   |   |   |   |       common.pb.go
|   |   |   |   |       crypto.pb.go
|   |   |   |   |       crypto_grpc.pb.go
|   |   |   |   |       deposit.pb.go
|   |   |   |   |       deposit_grpc.pb.go
|   |   |   |   |       transaction.pb.go
|   |   |   |   |       transaction_grpc.pb.go
|   |   |   |   |       wallet.pb.go
|   |   |   |   |       wallet_grpc.pb.go
|   |   |   |   |
|   |   |   |   +---receipt
|   |   |   |   |   \---v3
|   |   |   |   |           receipt_v3.pb.go
|   |   |   |   |           receipt_v3_grpc.pb.go
|   |   |   |   |
|   |   |   |   \---v1
|   |   |   |           account.pb.go
|   |   |   |           account_grpc.pb.go
|   |   |   |
|   |   |   \---notificationpb
|   |   |           notification.pb.go
|   |   |           notification_grpc.pb.go
|   |   |
|   |   +---smswhatsapppb
|   |   |       sms.pb.go
|   |   |       sms_grpc.pb.go
|   |   |
|   |   \---urbacpb
|   |           urbac.pb.go
|   |           urbac_grpc.pb.go
|   |
|   +---notification
|   |       factory.not.go
|   |
|   +---partner
|   |       factory.ptn.go
|   |
|   +---proto
|   |   +---admin
|   |   |       auth.proto
|   |   |       session.proto
|   |   |       urbac.proto
|   |   |
|   |   +---partner
|   |   |       auth.proto
|   |   |       session.proto
|   |   |       svc.proto
|   |   |       urbac.proto
|   |   |
|   |   +---shared
|   |   |   +---accounting
|   |   |   |       account.proto
|   |   |   |       receipt.proto
|   |   |   |       receipt_v2.proto
|   |   |   |       receipt_v3.proto
|   |   |   |
|   |   |   +---core
|   |   |   |       core.proto
|   |   |   |
|   |   |   +---email
|   |   |   |       email.proto
|   |   |   |
|   |   |   +---notification
|   |   |   |       notification.proto
|   |   |   |
|   |   |   +---otp
|   |   |   |       otp.proto
|   |   |   |
|   |   |   \---sms
|   |   |           sms.proto
|   |   |
|   |   \---user
|   |       +---account
|   |       |       account.proto
|   |       |
|   |       +---auth
|   |       |       auth.proto
|   |       |
|   |       +---session
|   |       |       session.proto
|   |       |
|   |       \---urbac
|   |               urbac.proto
|   |
|   +---response
|   |       response.go
|   |
|   +---secrets
|   |       jwt_public.pem
|   |
|   +---sms
|   |       factory.sms.go
|   |
|   +---urbac
|   |   |   factory.urbac.go
|   |   |
|   |   \---utils
|   |           rbac_utils.go
|   |
|   \---utils
|       +---cache
|       |       cache_util.go
|       |
|       +---errors
|       |       x.errors.go
|       |
|       +---id
|       |       id.generator.go
|       |       password_gen.go
|       |
|       +---image
|       |       compress.image.go
|       |
|       +---notification
|       |       utils.go
|       |
|       \---profile
|               profile_fetcher.go
|
+---ui
|   \---screen
|           account.html
|           cashier.html
|           dashboard.html
|           index.html
|           login.html
|           oauth2_consent.html
|           oauth2_login.html
|
\---uploads
```

---

## Folder Purpose

| Folder                | Purpose                                                                          |
| --------------------- | -------------------------------------------------------------------------------- |
| `services/`           | Each microservice is an isolated Go module (its own go.mod)                      |
| `proto/`              | Shared protobufs for gRPC; compiled per service into `/proto` folder inside each |
| `deployments/`        | Kubernetes manifests (can also support Helm/Kustomize later)                     |
| `configs/`            | App-specific configuration (env variables, secrets, etc.)                        |
| `scripts/`            | Build, test, and proto-generation scripts                                        |
| `docker-compose.yaml` | For spinning up services locally for testing                                     |
| `Makefile`            | For building, formatting, and generating code across services                    |

---

## Tools/Practices Supported

| Category               | Tools/Practices                                           |
| ---------------------- | --------------------------------------------------------- |
| Communication          | gRPC, Protobuf                                            |
| Deployment             | Docker, Kubernetes                                        |
| Service Discovery      | DNS via K8s service names                                 |
| Secrets                | K8s secrets or Vault                                      |
| Observability          | Prometheus, Grafana, Loki, Jaeger                         |
| CI/CD (later)          | GitHub Actions, ArgoCD, Helm                              |
| Rate Limiting          | Redis, API Gateway                                        |
| Auth                   | JWT, OAuth2 (Auth service)                                |
| Database (per service) | Postgres, Redis, TimescaleDB, etc. (each owns its own DB) |

---

## Service Example: `/services/auth-service/`

```
auth-service/
├── cmd/
│   └── main.go                 # Starts gRPC server
├── internal/
│   ├── domain/                 # Entities like User, Session
│   ├── usecase/                # Business logic (RegisterUser, LoginUser)
│   └── handler/                # gRPC/HTTP handlers (adapters)
├── proto/                      # Compiled .pb.go files
├── Dockerfile
├── go.mod
└── go.sum
```

---

## Adding a New Service Later?

Just:

1. `mkdir services/new-service`
2. `go mod init github.com/yourorg/pxyz/services/new-service`
3. Define its proto in `/proto/`
4. Implement handlers, usecases, domain
5. Add `Dockerfile` + `k8s` manifests

> It's fully isolated and ready to scale independently.

---

> Running docker 
```
cd services/auth-service
docker build -t auth-service .
docker run -p 8080:8080 auth-service
docker-compose up --build # build whole project
# run postgres sql file
psql -U postgres -d auth_db -f init.sql
```

---

## Step-by-Step Setup for Windows

### 1. Install `protoc` (Protocol Buffers Compiler)

1. Download `protoc` from:
   [https://github.com/protocolbuffers/protobuf/releases](https://github.com/protocolbuffers/protobuf/releases)

   * Get the latest release for **Windows (`protoc-<version>-win64.zip`)**
2. Extract the ZIP to:
   `C:\Program Files\protoc\`
3. Add `C:\Program Files\protoc\bin` to your **System PATH**:

   * Open Start Menu → search “Environment Variables” → Edit system variables → `Path` → Add

> Confirm it's working:

```bash
protoc --version
```

---

### 2. Install gRPC Plugins (Go)

Open **Command Prompt or PowerShell** and run:

```bash
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
```

> These executables will be placed in:

```
C:\Users\<YourUsername>\go\bin
```

Add that path to your **System PATH** so `protoc` can find them.

---

### 3. Generate Proto Files

Once setup is complete:

```bash
cd proto
protoc --go_out=. --go-grpc_out=. auth.proto
```

This will generate:

* `auth.pb.go`
* `auth_grpc.pb.go`

---

### 4. Install Docker Desktop for Windows

[https://www.docker.com/products/docker-desktop/](https://www.docker.com/products/docker-desktop/)

* Enable WSL2 backend (recommended).
* Confirm installation:

```bash
docker --version
```

---

### 5. (Optional) Install Kubernetes Tools

To run Kubernetes locally:

#### Option A: Minikube

```bash
choco install minikube
```

#### Option B: Kind (Kubernetes-in-Docker)

```bash
choco install kind
```

Install `kubectl` CLI:

```bash
choco install kubernetes-cli
```

---

### 6. VS Code Extensions

Install these from the **Extensions panel** (`Ctrl+Shift+X`):

| Extension Name          | Description                     |
| ----------------------- | ------------------------------- |
| Go                      | Official Go plugin              |
| Docker                  | Build & run containers visually |
| YAML                    | Helpful for Kubernetes configs  |
| Proto3 Language Support | Syntax highlight for .proto     |

---