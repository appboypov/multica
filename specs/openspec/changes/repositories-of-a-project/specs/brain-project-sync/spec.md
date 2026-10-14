## REMOVED Requirements

### Requirement: The repository project of a Product with one repository merges into its project
**Reason**: Its subject, a repository project carrying a checkout, is gone. UJ-1's run merged every such project, and this change drops project resources (597). A repository now reaches its Product's project as a repository, through `brain-repository-sync`.
**Migration**: None. The sync adds each repository of a Product to that Product's project, as `brain-repository-sync` "Each repository of a Product is a repository of its project" says. Its scenario "A repository without a Product" moves to `brain-repository-sync` "A repository the sync cannot add is reported and left alone".

### Requirement: The repository projects of a Product with several repositories stay
**Reason**: `brain-repository-sync` "Kept repository projects merge into their Product's project" merges the projects UJ-1 kept, once their repository is added.
**Migration**: None. A kept project stays only while its repository cannot be added, as that requirement's scenario "A repository the sync cannot add keeps its project" says.
