import FormData from "form-data"; // form-data v4.0.1
import Mailgun from "mailgun.js"; // mailgun.js v11.1.0

async function sendSimpleMessage() {
  if (!process.env.API_KEY) {
    throw new Error("API_KEY environment variable is required");
  }
  const mailgun = new Mailgun(FormData);
  const mg = mailgun.client({
    username: "api",
    key: process.env.API_KEY,
    // When you have an EU-domain, you must specify the endpoint:
    // url: "https://api.eu.mailgun.net"
  });
  try {
    const data = await mg.messages.create("sandbox986811d84fb842f9a4cd2084b6fcdecb.mailgun.org", {
      from: "Mailgun Sandbox <postmaster@sandbox986811d84fb842f9a4cd2084b6fcdecb.mailgun.org>",
      to: ["Jens Mittelbach <mail@jensmittelbach.de>"],
      subject: "Hello Jens Mittelbach",
      text: "Congratulations Jens Mittelbach, you just sent an email with Mailgun! You are truly awesome!",
    });

    console.log(data); // logs response data
  } catch (error) {
    console.log(error); //logs any error
  }
}