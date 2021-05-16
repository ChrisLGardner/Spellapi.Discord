param containerName string = 'spellapi.discord'
param containerVersion string = 'main'

@secure()
param Token string
param ImageRegistry string = 'halbarad.azurecr.io'
param ImageRegistryUsername string = 'halbarad'

@secure()
param ImageRegistryPassword string
param HoneycombDataset string

@secure()
param HoneycombApiKey string
param ApiUrl string

resource bot_aci 'Microsoft.ContainerInstance/containerGroups@2018-10-01' = {
  name: containerName
  location: 'westeurope'
  properties: {
    containers: [
      {
        name: containerName
        properties: {
          image: '${ImageRegistry}/go/${containerName}:${containerVersion}'
          ports: [
            {
              protocol: 'TCP'
              port: 80
            }
          ]
          environmentVariables: [
            {
              name: 'DISCORD_TOKEN'
              value: Token
            }
            {
              name: 'HONEYCOMB_KEY'
              value: HoneycombApiKey
            }
            {
              name: 'HONEYCOMB_DATASET'
              value: HoneycombDataset
            }
            {
              name: 'API_URL'
              value: ApiUrl
            }
          ]
          resources: {
            requests: {
              memoryInGB: '1.5'
              cpu: 1
            }
          }
        }
      }
    ]
    imageRegistryCredentials: [
      {
        server: ImageRegistry
        username: ImageRegistryUsername
        password: ImageRegistryPassword
      }
    ]
    restartPolicy: 'OnFailure'
    ipAddress: {
      type: 'Private'
    }
    osType: 'Linux'
  }
}
