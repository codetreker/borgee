# Content Lock: Helper Status UI And Current Sync

## Locked DOM Selectors And Attributes

- Helper status page root: `data-page="helper-status"`
- Status row/badge attribute: `data-helper-status="connected|offline|revoked|uninstalled|pending"`
- Allowed category attribute: `data-helper-category="<category>"`
- Sidebar action: `data-action="open-helper-status"`

## Locked Status Labels

- `Helper connected`
- `Helper offline`
- `Helper revoked`
- `Helper uninstalled`
- `Waiting for local Helper`

## Locked Safe Timestamp Text

- `Last seen`
- `No last seen yet`

## Locked Negative Copy

The Helper status UI must not render these success claims:

- `Configure OpenClaw succeeded`
- `OpenClaw connected`
- `job succeeded`

## Locked Category Labels

- `OpenClaw lifecycle`
- `OpenClaw config`
- `Helper lifecycle`
- `Status collection`

Unknown category values render as their raw unknown category string and remain non-actionable text.
