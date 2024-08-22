const cardValidator = require('simple-card-validator');
const { v4: uuidv4 } = require('uuid');
const pino = require('pino');

const logger = pino({
  name: 'paymentservice-charge',
  messageKey: 'message',
  changeLevelName: 'severity',
  useLevelLabels: true
});


class CreditCardError extends Error {
  constructor (message) {
    super(message);
    this.code = 400; // Invalid argument error
  }
}

class InvalidCreditCard extends CreditCardError {
  constructor (cardType) {
    super(`Credit card info is invalid`);
  }
}

class UnacceptedCreditCard extends CreditCardError {
  constructor (cardType) {
    super(`Sorry, we cannot process ${cardType} credit cards. Only VISA or MasterCard is accepted.`);
  }
}

class ExpiredCreditCard extends CreditCardError {
  constructor (number, month, year) {
    super(`Your credit card (ending ${number.substr(-4)}) expired on ${month}/${year}`);
  }
}

/**
 * Verifies the credit card number and (pretend) charges the card.
 *
 * @param {*} request
 * @return transaction_id - a random uuid v4.
 */
module.exports = async function charge (request) {
  const { amount, credit_card: creditCard } = request;
  const { credit_card_number: cardNumber, credit_card_expiration_year: year, credit_card_expiration_month: month } = creditCard;
  
  const cardInfo = cardValidator(cardNumber);
  const { card_type: cardType, valid } = cardInfo.getCardDetails();
  
  const currentDate = new Date();
  const currentMonth = currentDate.getMonth() + 1;
  const currentYear = currentDate.getFullYear();
  
  if (!valid) {
    logger.error('Invalid credit card detected');
    return { error: 'InvalidCreditCard', message: 'The credit card provided is invalid.' };
  }
  
  // Only VISA and Mastercard are accepted
  if (cardType !== 'visa' && cardType !== 'mastercard') {
    logger.error(`Unaccepted credit card type: ${cardType}`);
    return { error: 'UnacceptedCreditCard', message: `Credit card type ${cardType} is not accepted.` };
  }
  
  // Validate that the expiration date is in the future
  const currentMonthYear = currentYear * 12 + currentMonth;
  const expirationMonthYear = year * 12 + month;
  if (currentMonthYear > expirationMonthYear) {
    logger.error(`Expired credit card: ${cardNumber.replace('-', '')}, Expiry: ${month}/${year}`);
    return { error: 'ExpiredCreditCard', message: 'The credit card is expired.' };
  }
  
  logger.info(`Transaction processed: ${cardType} ending in ${cardNumber.slice(-4)} | Amount: ${amount.currency_code}${amount.units}.${amount.nanos}`);
  
  return { transaction_id: uuidv4() }
};
