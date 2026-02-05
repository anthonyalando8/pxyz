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

**CI/CD Implementation**

---
# 🚀 **Complete GitHub CI/CD Setup Guide**

## **Step 1: Create GitHub Personal Access Token (PAT)**

### 1.1 Generate PAT for GHCR
1. Go to GitHub → **Settings** → **Developer settings** → **Personal access tokens** → **Tokens (classic)**
2. Click **Generate new token (classic)**
3. Name it: `GHCR_PAT`
4. Select scopes:
   - ✅ `write:packages` (upload packages)
   - ✅ `read:packages` (download packages)
   - ✅ `delete:packages` (optional - cleanup old images)
   - ✅ `repo` (access private repos if needed)
5. Click **Generate token**
6. **⚠️ COPY THE TOKEN NOW** - you won't see it again!

---

## **Step 2: Generate SSH Deploy Key**

### 2.1 On Your Local Machine
```bash
# Generate SSH key pair
ssh-keygen -t ed25519 -C "deploy@safarigari.com" -f ~/.ssh/deploy_key

# This creates two files:
# - deploy_key (private key) - goes to GitHub Secrets
# - deploy_key.pub (public key) - goes to your server
```

### 2.2 Add Public Key to Server
```bash
# Copy public key to server
ssh-copy-id -i ~/.ssh/deploy_key.pub your_user@212.95.35.81

# OR manually:
ssh your_user@212.95.35.81
mkdir -p ~/.ssh
echo "your_public_key_content" >> ~/.ssh/authorized_keys
chmod 600 ~/.ssh/authorized_keys
chmod 700 ~/.ssh
```

### 2.3 Test SSH Connection
```bash
ssh -i ~/.ssh/deploy_key your_user@212.95.35.81
```

---

## **Step 3: Configure GitHub Repository Secrets**

### 3.1 Go to Repository Settings
1. Navigate to your GitHub repository
2. Click **Settings** → **Secrets and variables** → **Actions**
3. Click **New repository secret**

### 3.2 Add Required Secrets

#### **Infrastructure Secrets:**
| Secret Name | Value | Description |
|-------------|-------|-------------|
| `GHCR_PAT` | `ghp_xxxxxxxxxxxx` | GitHub token from Step 1.1 |
| `SERVER_IP` | `212.95.35.81` | Your server IP |
| `SERVER_USER` | `your_username` | SSH username on server |
| `DEPLOY_KEY` | `-----BEGIN OPENSSH PRIVATE KEY-----...` | Private key content from Step 2.1 |

#### **Database Secrets:**
| Secret Name | Value |
|-------------|-------|
| `DB_HOST` | `212.95.35.81` |
| `DB_PORT` | `5432` |
| `DB_USER` | `sam` |
| `DB_PASSWORD` | `your_db_password` |
| `REDIS_PASS` | `your_redis_password` (or empty) |

#### **Application Secrets:**
| Secret Name | Value |
|-------------|-------|
| `SYSTEM_ADMIN_PASSWORD` | `your_admin_password` |
| `SYSTEM_ADMIN_EMAIL` | `admin@example.com` |
| `JWT_SECRET` | `your_jwt_secret` |

#### **Email/SMS Secrets:**
| Secret Name | Value |
|-------------|-------|
| `SMTP_HOST` | `mail.derinance.com` |
| `SMTP_USER` | `no_reply@derinance.com` |
| `SMTP_PASS` | `your_smtp_password` |
| `SMTP_PORT` | `465` |
| `SMS_KEY` | `your_sms_key` |
| `WA_KEY` | `your_whatsapp_key` |

#### **Payment Gateway Secrets:**
| Secret Name | Value |
|-------------|-------|
| `MPESA_CONSUMER_KEY` | `your_mpesa_key` |
| `MPESA_CONSUMER_SECRET` | `your_mpesa_secret` |
| `MPESA_PASSKEY` | `your_mpesa_passkey` |
| `MPESA_SHORT_CODE` | `174379` |
| `B2C_CONSUMER_KEY` | `your_b2c_key` |
| `B2C_CONSUMER_SECRET` | `your_b2c_secret` |

#### **Monitoring Secrets:**
| Secret Name | Value |
|-------------|-------|
| `GRAFANA_PASSWORD` | `your_grafana_password` |

---

## **Step 4: Configure GitHub Repository Variables (Optional)**

For non-sensitive configuration that you want visible in logs:

1. Go to **Settings** → **Secrets and variables** → **Actions** → **Variables**
2. Click **New repository variable**

| Variable Name | Value |
|---------------|-------|
| `DOCKER_REGISTRY` | `ghcr.io` |
| `ENVIRONMENT` | `production` |
| `LOG_LEVEL` | `info` |

---

## **Step 5: Prepare Server**

### 5.1 SSH into Server
```bash
ssh your_user@212.95.35.81
```

### 5.2 Install Docker & Docker Compose
```bash
# Update system
sudo apt update && sudo apt upgrade -y

# Install Docker
curl -fsSL https://get.docker.com -o get-docker.sh
sudo sh get-docker.sh

# Add user to docker group
sudo usermod -aG docker $USER

# Install Docker Compose
sudo curl -L "https://github.com/docker/compose/releases/latest/download/docker-compose-$(uname -s)-$(uname -m)" -o /usr/local/bin/docker-compose
sudo chmod +x /usr/local/bin/docker-compose

# Verify installations
docker --version
docker-compose --version

# Log out and back in for group changes to take effect
exit
ssh your_user@212.95.35.81
```

### 5.3 Create Project Directory
```bash
# Create deployment directory
sudo mkdir -p /var/www/user.safarigari.com
sudo chown -R $USER:$USER /var/www/user.safarigari.com
cd /var/www/user.safarigari.com
```

### 5.4 Create Production Docker Compose File
```bash
nano docker-compose.prod.yml
```

Paste this content:
```yaml
version: '3.9'

services:
  traefik:
    image: traefik:v3.0
    container_name: traefik
    # ... (use the optimized compose from previous response)

  # ... all other services
```

### 5.5 Create Environment File
```bash
nano .env
```

Add:
```bash
# Docker Registry
DOCKER_REGISTRY=ghcr.io
GITHUB_REPOSITORY=your-username/your-repo
IMAGE_TAG=latest

# Database
DB_HOST=212.95.35.81
DB_PORT=5432
DB_USER=sam
DB_PASSWORD=your_password
DB_NAME=pxyz_user

# Redis
REDIS_PASS=

# ... other variables
```

### 5.6 Create Required Directories
```bash
mkdir -p uploads
mkdir -p services/common-services/authentication/auth-service/secrets
mkdir -p services/common-services/authentication/session-mngt/secrets
# ... create all secrets directories

# Create shared volumes
mkdir -p data/redis
mkdir -p data/kafka
mkdir -p data/prometheus
mkdir -p data/grafana
```

### 5.7 Copy JWT Keys
```bash
# Generate JWT keys if you don't have them
openssl genrsa -out jwt_private.pem 4096
openssl rsa -in jwt_private.pem -pubout -out jwt_public.pem

# Copy to all services that need them
cp jwt_*.pem services/common-services/authentication/auth-service/secrets/
cp jwt_*.pem services/common-services/authentication/session-mngt/secrets/
# ... copy to other services
```

---

## **Step 6: Configure GitHub Actions Workflow**

### 6.1 Create Workflow File
In your repository, create:
```
.github/workflows/deploy.yml
```

Use the workflow from my previous response.

### 6.2 Update Workflow Environment Variables
Add this to the top of your workflow:
```yaml
env:
  DOCKER_REGISTRY: ghcr.io
  IMAGE_PREFIX: ${{ github.repository }}
```

---

## **Step 7: Configure Package Permissions**

### 7.1 Enable Package Visibility
1. Go to your repository
2. Click on **Packages** (in right sidebar)
3. Select each package
4. Click **Package settings**
5. Set visibility to **Public** or **Private**
6. Add repository access under **Manage Actions access**

### 7.2 Link Packages to Repository
After first build, link packages:
1. Go to **Packages** → Select package
2. Click **Connect repository**
3. Select your repository

---

## **Step 8: Create Pre-Deployment Checklist**

### 8.1 Server Checklist
```bash
# On server, create checklist script
nano ~/pre-deploy-check.sh
```

```bash
#!/bin/bash

echo "🔍 Pre-deployment Checklist"
echo "================================"

# Check Docker
if docker --version &> /dev/null; then
    echo "✅ Docker installed"
else
    echo "❌ Docker not installed"
    exit 1
fi

# Check Docker Compose
if docker-compose --version &> /dev/null; then
    echo "✅ Docker Compose installed"
else
    echo "❌ Docker Compose not installed"
    exit 1
fi

# Check directory
if [ -d "/var/www/user.safarigari.com" ]; then
    echo "✅ Deployment directory exists"
else
    echo "❌ Deployment directory missing"
    exit 1
fi

# Check docker-compose.prod.yml
if [ -f "/var/www/user.safarigari.com/docker-compose.prod.yml" ]; then
    echo "✅ docker-compose.prod.yml exists"
else
    echo "❌ docker-compose.prod.yml missing"
    exit 1
fi

# Check .env
if [ -f "/var/www/user.safarigari.com/.env" ]; then
    echo "✅ .env file exists"
else
    echo "⚠️  .env file missing (optional)"
fi

# Check disk space
DISK_USAGE=$(df -h / | awk 'NR==2 {print $5}' | sed 's/%//')
if [ $DISK_USAGE -lt 80 ]; then
    echo "✅ Sufficient disk space ($DISK_USAGE% used)"
else
    echo "⚠️  Low disk space ($DISK_USAGE% used)"
fi

# Check memory
FREE_MEM=$(free -m | awk 'NR==2 {printf "%.0f", $7}')
if [ $FREE_MEM -gt 1000 ]; then
    echo "✅ Sufficient memory (${FREE_MEM}MB free)"
else
    echo "⚠️  Low memory (${FREE_MEM}MB free)"
fi

echo "================================"
echo "✅ Server ready for deployment"
```

```bash
chmod +x ~/pre-deploy-check.sh
./pre-deploy-check.sh
```

---

## **Step 9: Test Deployment**

### 9.1 Local Test (Optional)
```bash
# Build and test locally first
docker-compose -f docker-compose.prod.yml build
docker-compose -f docker-compose.prod.yml up -d
docker-compose -f docker-compose.prod.yml ps

# Check logs
docker-compose -f docker-compose.prod.yml logs -f auth-service

# Stop
docker-compose -f docker-compose.prod.yml down
```

### 9.2 Push to GitHub
```bash
git add .
git commit -m "feat: add CI/CD pipeline"
git push origin main
```

### 9.3 Monitor Workflow
1. Go to GitHub repository
2. Click **Actions** tab
3. Watch the workflow run
4. Check each step for errors

### 9.4 Verify Deployment
```bash
# SSH into server
ssh your_user@212.95.35.81

# Check running containers
docker ps

# Check specific service
docker logs -f auth-service

# Test endpoints
curl http://localhost/api/v1/auth/health
curl http://localhost:8080  # Traefik dashboard
```

---

## **Step 10: Setup Secrets Management Script**

### 10.1 Create Secrets Helper
```bash
# On your local machine
nano setup-github-secrets.sh
```

```bash
#!/bin/bash

REPO="your-username/your-repo"

# Function to add secret
add_secret() {
    local name=$1
    local value=$2
    
    gh secret set "$name" --body "$value" --repo "$REPO"
    echo "✅ Added secret: $name"
}

echo "🔐 Setting up GitHub Secrets for $REPO"
echo "========================================"

# Read secrets from user input
read -sp "Enter GHCR_PAT: " GHCR_PAT && echo
read -p "Enter SERVER_IP: " SERVER_IP
read -p "Enter SERVER_USER: " SERVER_USER
read -sp "Enter DB_PASSWORD: " DB_PASSWORD && echo
read -sp "Enter SYSTEM_ADMIN_PASSWORD: " SYSTEM_ADMIN_PASSWORD && echo

# Add secrets
add_secret "GHCR_PAT" "$GHCR_PAT"
add_secret "SERVER_IP" "$SERVER_IP"
add_secret "SERVER_USER" "$SERVER_USER"
add_secret "DB_PASSWORD" "$DB_PASSWORD"
add_secret "SYSTEM_ADMIN_PASSWORD" "$SYSTEM_ADMIN_PASSWORD"

# Read deploy key from file
if [ -f ~/.ssh/deploy_key ]; then
    DEPLOY_KEY=$(cat ~/.ssh/deploy_key)
    add_secret "DEPLOY_KEY" "$DEPLOY_KEY"
else
    echo "⚠️  Deploy key not found at ~/.ssh/deploy_key"
fi

echo "========================================"
echo "✅ Secrets setup complete!"
```

```bash
chmod +x setup-github-secrets.sh

# Install GitHub CLI if not installed
# For Ubuntu/Debian:
curl -fsSL https://cli.github.com/packages/githubcli-archive-keyring.gpg | sudo dd of=/usr/share/keyrings/githubcli-archive-keyring.gpg
echo "deb [arch=$(dpkg --print-architecture) signed-by=/usr/share/keyrings/githubcli-archive-keyring.gpg] https://cli.github.com/packages stable main" | sudo tee /etc/apt/sources.list.d/github-cli.list > /dev/null
sudo apt update
sudo apt install gh

# Login to GitHub CLI
gh auth login

# Run the script
./setup-github-secrets.sh
```

---

## **Step 11: Create Monitoring & Alerts**

### 11.1 Setup Health Check Script (On Server)
```bash
nano /var/www/user.safarigari.com/health-check.sh
```

```bash
#!/bin/bash

SERVICES=(
    "traefik"
    "redis"
    "kafka"
    "auth-service"
    "session-service"
    "email-service"
    "core-service"
    "payment-service"
    "crypto-service"
)

echo "🏥 Health Check - $(date)"
echo "================================"

FAILED=0

for SERVICE in "${SERVICES[@]}"; do
    if docker ps | grep -q "$SERVICE"; then
        STATUS=$(docker inspect --format='{{.State.Health.Status}}' $SERVICE 2>/dev/null || echo "running")
        if [ "$STATUS" == "healthy" ] || [ "$STATUS" == "running" ]; then
            echo "✅ $SERVICE: $STATUS"
        else
            echo "❌ $SERVICE: $STATUS"
            FAILED=$((FAILED + 1))
        fi
    else
        echo "❌ $SERVICE: not running"
        FAILED=$((FAILED + 1))
    fi
done

echo "================================"
if [ $FAILED -eq 0 ]; then
    echo "✅ All services healthy"
    exit 0
else
    echo "❌ $FAILED service(s) unhealthy"
    exit 1
fi
```

```bash
chmod +x /var/www/user.safarigari.com/health-check.sh

# Add to crontab (run every 5 minutes)
crontab -e

# Add this line:
*/5 * * * * /var/www/user.safarigari.com/health-check.sh >> /var/log/health-check.log 2>&1
```

---

## **Step 12: Create Rollback Script**

```bash
nano /var/www/user.safarigari.com/rollback.sh
```

```bash
#!/bin/bash

echo "🔄 Rolling back to previous version..."

cd /var/www/user.safarigari.com

# Stop current containers
docker-compose -f docker-compose.prod.yml down

# Pull previous image version
# This assumes you tagged previous version
docker-compose -f docker-compose.prod.yml pull

# Start with previous images
docker-compose -f docker-compose.prod.yml up -d

echo "✅ Rollback complete"
```

```bash
chmod +x /var/www/user.safarigari.com/rollback.sh
```

---

## **Step 13: Documentation**

### 13.1 Create DEPLOYMENT.md
```markdown
# Deployment Guide

## Prerequisites
- Docker & Docker Compose installed
- GitHub secrets configured
- Server prepared

## Manual Deployment
```bash
cd /var/www/user.safarigari.com
docker-compose -f docker-compose.prod.yml pull
docker-compose -f docker-compose.prod.yml up -d
```

## Rollback
```bash
./rollback.sh
```

## Health Check
```bash
./health-check.sh
```

## Logs
```bash
# All services
docker-compose -f docker-compose.prod.yml logs -f

# Specific service
docker logs -f auth-service
```
```

---

## **✅ Final Checklist**

- [ ] GitHub PAT created and added to secrets
- [ ] SSH deploy key generated and added
- [ ] All required secrets added to GitHub
- [ ] Server has Docker & Docker Compose installed
- [ ] Project directory created on server
- [ ] docker-compose.prod.yml on server
- [ ] .env file configured on server
- [ ] JWT keys generated and distributed
- [ ] Secrets directories created
- [ ] Workflow file committed to repository
- [ ] Package permissions configured
- [ ] Health check script setup
- [ ] Rollback script ready
- [ ] First deployment tested

---

## **🚀 Deploy!**

```bash
git add .
git commit -m "feat: complete CI/CD setup"
git push origin main
```

Monitor the deployment at:
```
https://github.com/your-username/your-repo/actions
```
