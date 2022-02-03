# Spellapi.Discord
Discord bot for interacting with the spellapi api

## Available commands:

|Command|Description|
|-|-|
|?help|displays the help information for all commands|
|?spell `<spell name>`|Finds the spell specified if possible. when there are multiple spells matching you can narrow it down using filters like "system=dnd" or "level: 2"|
|?spell add| Requires an attachment to have been added to the message. Will upload the attachment to the backend and make it searchable. See below for the format of the attachment.|

## Spell Attachment format

Spells added via the bot need to be done using a file attachment. This file needs to be in a JSON format and look like below:

```
{
    "name": "fireball",
    "description": "Deals 3 levels of Fire damage to all enemies within 10m of the target point.",
    "spelldata":{
        "level": 2,
        "school": "evocation",
        "type": "fire"
    }
    "metadata":{
        "system":"test1"
    }
}
```

The Name and Description are both required fields, along with the System in the metadata block. Spelldata can be anything you want/need for the spell that isn't directly tied to it's name or description, as seen here where it's used for level, school etc but could also be related to duration or skills needed or anything else as appropriate. This flexibility allows it to also be used for spell-like effects, such as psychic powers or martial maneuvers etc.
