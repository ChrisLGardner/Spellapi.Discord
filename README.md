# Spellapi.Discord
Discord bot for interacting with the spellapi api

## Available commands:

|Command|Description|
|-|-|
|?help|displays the help information for all commands|
|?spell `<spell name>`|Finds the spell specified if possible. when there are multiple spells matching you can narrow it down using filters like "system=dnd" or "level: 2"|
|?spell add| Will add a new spell, either using the content of the message or via attachment. See below for the format of the message or attachment.|

## Spell Attachment format

Spells added via the bot can be done either using the body of the message or using a file attachment.

Message content should be in the following format:

```
name: Example Spell 6
description: Target creature you can see falls prone and cannot act until after your next turn.
spelldata:
    level: 2
metadata:
    system: test1
```

Description can support Markdown using the same format as Discord, it's recommended to limit usage to **bold**, _italics_ and other similar effects due to the limited space of the returned message. It can also support multiline using the following format:

```
description: |
  This is a multiline description.
  The pipe character is required.
```

File attachments can either be in the format above or using the JSON format below:

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
