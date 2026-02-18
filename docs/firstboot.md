# First Boot Setup

When AvyOS boots for the first time, it launches the Welcome application to guide you through initial system configuration.

## Overview

The first boot wizard configures:

1. **Language** — System display language
2. **Timezone** — Local time zone
3. **User Account** — Primary user creation
4. **Password** — Account security

After completion, the system is ready to use and will boot directly to the login screen on subsequent starts.

## Setup Pages

### 1. Welcome

```
    ╭─────────────────────────────────────╮
    │                                     │
    │         Welcome to AvyOS            │
    │                                     │
    │   A Linux-based operating system    │
    │   written entirely in pure Go.      │
    │                                     │
    │         [ Get Started ]             │
    │                                     │
    ╰─────────────────────────────────────╯
```

Press **Enter** or click **Get Started** to begin setup.

### 2. Language Selection

Select your preferred system language:

| Language     | Code  |
| ------------ | ----- |
| English (US) | en_US |
| English (UK) | en_GB |
| Deutsch      | de_DE |
| Francais     | fr_FR |
| Espanol      | es_ES |
| Italiano     | it_IT |
| Portugues    | pt_BR |
| Русский      | ru_RU |
| 日本語       | ja_JP |
| 中文         | zh_CN |
| 한국어       | ko_KR |
| हिन्दी          | hi_IN |

Use **Arrow Keys** to navigate, **Enter** to select.

### 3. Timezone

Select your timezone:

| Region              | UTC Offset |
| ------------------- | ---------- |
| Pacific/Honolulu    | UTC-10     |
| America/Los_Angeles | UTC-8      |
| America/Denver      | UTC-7      |
| America/Chicago     | UTC-6      |
| America/New_York    | UTC-5      |
| America/Sao_Paulo   | UTC-3      |
| Atlantic/Reykjavik  | UTC+0      |
| Europe/London       | UTC+0      |
| Europe/Paris        | UTC+1      |
| Europe/Moscow       | UTC+3      |
| Asia/Dubai          | UTC+4      |
| Asia/Kolkata        | UTC+5:30   |
| Asia/Shanghai       | UTC+8      |
| Asia/Tokyo          | UTC+9      |
| Australia/Sydney    | UTC+11     |

Use **Arrow Keys** to navigate, **Enter** to select.

### 4. User Account

Create your user account:

```
    Full Name: [Alice Smith              ]

    Username:  alicesmith (auto-generated)
```

- **Full Name** — Your display name
- **Username** — Auto-generated from full name (lowercase, no spaces)

The username must:
- Be lowercase letters and numbers only
- Start with a letter
- Be 3-32 characters long

Press **Enter** to continue after entering your name.

### 5. Password

Set your account password:

```
    Password:         [••••••••••••        ]
    Confirm Password: [••••••••••••        ]

    Strength: Strong ████████░░
```

Password requirements:
- Minimum 8 characters
- Both fields must match

The strength indicator shows:
- **Weak** — Less than 8 characters
- **Medium** — 8-11 characters
- **Strong** — 12+ characters

Press **Enter** to continue after confirming your password.

### 6. Setup Complete

```
    ╭─────────────────────────────────────╮
    │        Setup Complete!              │
    │                                     │
    │   Language:  English (US)           │
    │   Timezone:  America/New_York       │
    │   Username:  alicesmith             │
    │                                     │
    │      [ Start Using AvyOS ]          │
    │                                     │
    ╰─────────────────────────────────────╯
```

Review your settings and press **Enter** to complete setup.

## What Happens During Setup

When you complete the wizard, AvyOS:

1. **Creates directories:**
   - `/apps/` — Application data
   - `/users/<username>/` — Your home directory
   - `/config/security/` — Security configuration
   - `/linux/` — Linux compatibility containers
   - `/volumes/` — Data volumes

2. **Creates your user account:**
   - Adds identity to `/config/security/identity.conf`
   - Sets up authentication in `/config/security/auth.conf`
   - Assigns default capabilities

3. **Marks setup complete:**
   - Creates `/config/.firstboot-done`
   - System boots to login screen next time

## Navigation

| Key            | Action                    |
| -------------- | ------------------------- |
| **Tab**        | Move to next field        |
| **Shift+Tab**  | Move to previous field    |
| **Enter**      | Select/Continue           |
| **Arrow Keys** | Navigate lists            |
| **Backspace**  | Delete character          |
| **Escape**     | Go back (where available) |

## Skipping First Boot

If first boot has already completed, the welcome wizard will not appear. The marker file `/config/.firstboot-done` indicates setup is complete.

To re-run first boot setup (for testing):
```
delete /config/.firstboot-done
power reboot
```

## Troubleshooting

### Setup doesn't appear

If the login screen appears instead of setup:
- Check if `/config/.firstboot-done` exists
- Delete it and reboot to re-run setup

### Can't type in fields

- Ensure the input field is focused (highlighted border)
- Press **Tab** to move between fields

### Password doesn't match

- Both password fields must be identical
- Clear both fields and re-enter carefully

### Username invalid

- Only lowercase letters and numbers allowed
- Must start with a letter
- Must be 3-32 characters
