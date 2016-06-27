# Query

Common Stat for DB

## Number of groups using `Jung2bot` in last 7 days

```javascript
db.getCollection('messages').distinct('chatId', {
  dateCreated: {
    $gte: new Date('2016-05-16T15:06:24.710Z') // time D-7
  }
})
```
