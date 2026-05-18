# Content Lock: ACL Forbidden-State UX

## UI Literals

- ArtifactPanel forbidden: `You do not have access to this artifact.`
- ArtifactComments loading: `Loading comments...`
- ArtifactComments forbidden: `You do not have access to these comments.`
- ArtifactComments unavailable: `Comments are unavailable right now.`
- Settings PermissionsView forbidden: `无权查看授权`
- Existing Settings `PermissionsView` literals remain available for their existing states: `加载中`, `加载失败`, `暂无授权`, `完整能力`.

## DOM Anchors

- ArtifactPanel forbidden state: `[data-artifact-forbidden]`
- ArtifactComments loading state: `[data-cv5-loading]`
- ArtifactComments forbidden state: `[data-cv5-forbidden]`
- ArtifactComments unavailable state: `[data-cv5-unavailable]`
- Settings PermissionsView forbidden state: `[data-ap2-forbidden]`

## Locked Absences

- Do not render server error bodies in denied-state copy.
- Do not expose protected channel names, artifact titles, message bodies, file names, permission grant internals, or audit records from denied responses.
- Do not add Task4 e2e scope, sidebar/footer IA, avatar/account panel changes, Helper/Remote Nodes placement, or new privacy/compliance product terms.
