import java.io.File;
import com.mashape.unirest.http.HttpResponse; // unirest v1.4.9
import com.mashape.unirest.http.JsonNode;
import com.mashape.unirest.http.Unirest;
import com.mashape.unirest.http.exceptions.UnirestException;
public class MGSamples {
  public static JsonNode sendSimpleMessage() throws UnirestException {
    String apiKey = System.getenv("API_KEY");
        if (apiKey == null) {
            throw new IllegalStateException("API_KEY environment variable is required");
        }

    HttpResponse<JsonNode> request = Unirest.post("https://api.mailgun.net/v3/sandbox986811d84fb842f9a4cd2084b6fcdecb.mailgun.org/messages")
      .basicAuth("api", apiKey)
      .queryString("from", "Mailgun Sandbox <postmaster@sandbox986811d84fb842f9a4cd2084b6fcdecb.mailgun.org>")
      .queryString("to", "Jens Mittelbach <mail@jensmittelbach.de>")
      .queryString("subject", "Hello Jens Mittelbach")
      .queryString("text", "Congratulations Jens Mittelbach, you just sent an email with Mailgun! You are truly awesome!")
      .asJson();
    return request.getBody();
  }