# Query

Common Stat for DB

## Number of groups using `Jung2bot` in last 7 days

```javascript
// basic distinct count
db.getCollection('messages').distinct('chatId', {
  dateCreated: {
    $gte: new Date('2016-09-17T15:02:43Z') // time D-7
  }
});

// detail stats
var cursor = db.getCollection('messages').aggregate([
    { $match: {
      dateCreated: {
        $gte: new Date('2016-09-17T15:02:43Z') // time D-7
      }
    }},
    { $group: {
        _id: "$chatId",
        chatTitle: { $last: '$chatTitle' },
        count: { $sum: 1 }
    }},
    { $project: {
        chatTitle: '$chatTitle',
        count: '$count'
    }},
    { $sort : { count : -1 } }
]);
while (cursor.hasNext()) {
   print(cursor.next());
}
```
