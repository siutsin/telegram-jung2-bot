const main = async () => {
  const current = new Date()
  const min = current.getMinutes()
  const remainder = min % parseInt(process.env.CRON_INTERVAL)
  current.setMilliseconds(0)
  current.setSeconds(0)
  current.setMinutes(min - remainder)

  const timeString = current.toISOString()
  const url = new URL(process.env.OFF_FROM_WORK_URL)
  url.searchParams.append('timeString', timeString)

  try {
    const response = await fetch(url, {
      method: 'GET'
    })

    if (!response.ok) {
      console.error(`HTTP error! status: ${response.status}`)
      return
    }

    const responseBody = await response.json()
    console.log({
      requestUrl: url.toString(),
      responseHeaders: response.headers,
      responseBody
    })
  } catch (e) {
    console.error(e.toLocaleString())
  }
}

main().catch(e => console.error(e.toLocaleString()))
