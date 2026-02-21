# OOBE (First Boot)

OOBE is the out-of-box experience shown when AvyOS starts with no completed first-boot marker.

It guides initial user setup before the normal login flow.

## OOBE flow

Typical flow:

1. Welcome
2. Language selection
3. Timezone selection
4. User profile creation
5. Password setup
6. Completion and handoff to login/desktop

## What OOBE configures

- primary user identity
- initial authentication credentials
- locale and timezone preferences
- first-boot completion marker

## Persisted state

During first boot, AvyOS writes configuration under `config/` and user state under `users/`.

After completion, a marker file indicates that OOBE is done so future boots go directly to login.

## Re-running OOBE for testing

If you need to test OOBE again on a development image:

```bash
delete /config/.firstboot-done
power reboot
```

## Troubleshooting

### OOBE does not appear

- verify `/config/.firstboot-done` does not exist
- confirm the image includes the OOBE app and login service

### Cannot continue from setup page

- check keyboard focus (Tab/Shift+Tab)
- verify required fields are filled (name, username, password)

### Login fails after OOBE

- verify identity/auth files were written under `config/security/`
- inspect service logs if available
