"""Bootstrap a running GlitchTip for e2e testing and manual inspection.

Run inside the web container:
    python manage.py shell -c "$(cat test/bootstrap.py)"

It is idempotent. On each run it:

  - creates (or reuses) a superuser whose email is marked verified, so it can be
    used to log in to the GlitchTip web UI directly (no email-confirmation step);
  - (re)creates a fully-scoped API token for that user;

and prints three lines that the e2e runner captures:

    GLITCHTIP_LOGIN_EMAIL=<email>
    GLITCHTIP_LOGIN_PASSWORD=<password>
    GLITCHTIP_TOKEN=<64-hex-char-token>

Override the credentials with the E2E_EMAIL / E2E_PASSWORD environment variables.
"""

import os

from django.contrib.auth import get_user_model

from apps.api_tokens.models import APIToken

# Every scope GlitchTip's APIToken BitField supports, so the token can manage
# organizations, teams, projects, keys, alerts, members, and monitors.
ALL_SCOPES = [
    "project:read", "project:write", "project:admin", "project:releases",
    "team:read", "team:write", "team:admin",
    "event:read", "event:write", "event:admin",
    "org:read", "org:write", "org:admin",
    "member:read", "member:write", "member:admin",
]

email = os.environ.get("E2E_EMAIL", "e2e@example.com")
password = os.environ.get("E2E_PASSWORD", "e2e-password-12345!")

User = get_user_model()
user = User.objects.filter(email=email).first()
if user is None:
    user = User.objects.create_user(email=email, password=password)

# Reset the password on every run and grant full admin so the account can manage
# everything and reach the Django admin (when ENABLE_ADMIN=True).
user.set_password(password)
user.is_active = True
user.is_staff = True
user.is_superuser = True
user.save()

# Mark the email verified so the GlitchTip web UI allows password login without
# an email-confirmation step (the confirmation email would otherwise only print
# to the container console because EMAIL_URL=consolemail://). Best-effort: the
# API token works regardless of whether allauth is present.
try:
    from allauth.account.models import EmailAddress

    EmailAddress.objects.update_or_create(
        user=user,
        email=email,
        defaults={"verified": True, "primary": True},
    )
except Exception as exc:  # pragma: no cover - best effort
    print("WARNING: could not mark email verified: %s" % exc)

# Replace any previous e2e token so the run starts from a known state.
APIToken.objects.filter(user=user, label="terraform-e2e").delete()
token = APIToken.objects.create(user=user, label="terraform-e2e")
token.add_permissions(ALL_SCOPES)
token.save()

print("GLITCHTIP_LOGIN_EMAIL=" + email)
print("GLITCHTIP_LOGIN_PASSWORD=" + password)
print("GLITCHTIP_TOKEN=" + token.token)
