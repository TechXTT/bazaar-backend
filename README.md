# bazaar-backend

## Repo structure

```
├─.env          # Environment variables
├─go.mod        # Go module file
├─go.sum
├─Escrow.json   # ABI file for the Escrow contract
├─main.go       # Main entry point for the application
├─bin/          # Binary files
├─modules/      # Modules that each represent a unit of independent logic
│ ├─products/     
│ ├─stores/     
│ ├─users/      
├─services/     # Services which are auto-registered as dependencies
│ ├─jwt/
│ ├─middleware/
│ ├─wsclient/
│ ├─config/
│ ├─web/
│ ├─observer/
│ ├─db/
│ ├─s3spaces/
├─pkg/
│ ├─app/        # Application logic
```