const runSolver = async (difficulty, prefix) => {
  const solver = new powSolver() // skipcq: JS-0125
  const nonce = await solver.solve(difficulty, prefix)
  let strNonce = nonce.toString('hex')
  document.querySelector('.calculating td.blink').innerHTML = 'V'
  document.querySelector('.calculating td.blink').classList.remove('blink')
  document.querySelector('.submitting').style.display = 'table-row'
  return strNonce
}

const sendResult = async (difficulty,prefix,nonce, redirect) => {
  try {
    body =  JSON.stringify({
        difficulty:difficulty,
        prefix:prefix,
        buffer:nonce})
    const response = await fetch(`${backend}/pow`, {
      method: 'POST',
      body: body,
    })
    window.responseStatus=response.status
    window.nonceSent=true
    if (response.status === 200) {
      document.querySelector('.submitting td.blink').innerHTML = 'V'
      document.querySelector('.submitting td.blink').classList.remove('blink')
      setInterval(() => {
        document.querySelector('.success').style.display = 'table-row'
      }, 500)
      setInterval(() => {
        if (redirect) {
          window.location.href = `${redirect}`
        } else {
          window.location.href = '/public'
        }
      }, 1000)
    } else {
      document.querySelector('.submitting td.blink').innerHTML = 'X'
      document.querySelector('.submitting td.blink').classList.remove('blink')
      setInterval(() => {
        document.querySelector('.failed').style.display = 'table-row'
      }, 500)
      setInterval(() => {
        window.location.reload()
      }, 3000)
    }
  } catch (err) {
    console.log('Error')
  }
}

// Real consumable api example
const checkRequest = async ()=>{
  const response = await fetch(`${backend}/health`,{ // PROXY API REQUEST
    method: 'GET',
    credentials:'include'
  })
  if (response.status == 200) {
    respResult =  await response.text()
    console.log(respResult)
  }
}
const getProblem = async ()=>{
  const response = await fetch(`${backend}/pow`,{
    method: 'GET'
  })
  if (response.status == 200) {
    jsonResult =  await response.json()
    initSolver(jsonResult.difficulty, jsonResult.prefix, "/welcome")
  }
}

const initSolver = (difficulty, prefix, redirect) => {
  setTimeout(async () => {
    const nonce = await runSolver(difficulty, prefix)
    setTimeout(async () => {
      await sendResult(difficulty,prefix, nonce, redirect)
    }, 100)
  }, 100)
}

window.init = () =>{
  setTimeout(async () => {
    await getProblem()
  }, 100)
}