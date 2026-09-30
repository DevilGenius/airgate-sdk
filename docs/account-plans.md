# Account plan contract

Plugins declare recognized OAuth plans in `sdk.PluginInfo.AccountPlans`.
The declaration travels through protobuf `PluginInfoResponse.account_plans`
(field 18). Core reads this explicit contract for credential filtering and group
model policies. `Metadata["account.oauth_plans"]` is not a fallback.

An `sdk.AccountPlan` contains:

- `Key`: stable lowercase policy key, using letters, digits and underscores.
- `Label`: display name; defaults to the key.
- `CredentialKey`: the stored credential field; defaults to `plan_type`.
- `MatchMode`: `AccountPlanExact` (default), `AccountPlanContains`, or
  `AccountPlanNormalizedContains`.
- `Matches`: accepted values; defaults to the key. Multiple values are ORed.

Exact and substring matching are case-sensitive. Normalized substring matching
lowercases both sides and removes characters outside ASCII letters and digits.
All declarations for one platform must use the same credential field. Core
validates declarations before publishing a candidate plugin generation; invalid
keys, reserved keys, duplicate normalized keys, invalid modes and empty matching
values prevent publication. Existing active generations are not replaced.

## Unknown and missing declarations

Core generates `unknown` itself. It matches missing/empty values and the complement
of the declared known-plan rules. Plugins must not declare `unknown`, `none`,
`oauth` or `apikey` as plan keys. Group model policies retain the existing
`oauth` key for OAuth Unknown; API-key accounts never match it.

Omitting `AccountPlans` means no recognized plans, not a vendor-specific default.
Core consequently exposes only OAuth Unknown. Declaring rules does not fetch or
infer a user's subscription: a plugin must separately obtain and return the real
credential value. In particular, this contract does not add plan detection to the
Claude plugin.

## Dependencies and updates

Core and plugins depend on the SDK; Core does not import concrete gateway plugin
packages. Core exposes resolved plans through the explicit API `account_plans`
field, which the frontend consumes without parsing metadata JSON. Plugin
publication/removal updates cached route classifications outside the request
path.

Deploy the updated SDK with rebuilt Core and declaring plugins together. Older
plugins omit the new protobuf field and are treated as having no known plans;
their legacy metadata cannot grant a known-plan routing classification.
