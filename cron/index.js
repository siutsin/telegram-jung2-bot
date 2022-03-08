import got from 'got'

const main = async () => {
  const current = new Date()
  const min = current.getMinutes()
  const reminder = min % parseInt(process.env.CRON_INTERVAL)
  current.setMilliseconds(0)
  current.setSeconds(0)
  current.setMinutes(min - reminder)

  const timeString = current.toISOString()
  const { request, headers, body } = await got(process.env.OFF_FROM_WORK_URL, { searchParams: { timeString } })
  console.log({
    requestUrl: request.requestUrl.toJSON(),
    responseHeaders: headers,
    responseBody: JSON.parse(body)
  })
}

main().catch(e => console.error(e.toLocaleString()))
